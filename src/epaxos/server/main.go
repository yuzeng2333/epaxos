package main

import "time"
import "os"
import "sync"
import "errors"
import "fmt"
import "log"
import "strconv"
import "net"
import "net/rpc"
import "epaxos/common"

var VERSION string

type InstState int32
type LeaderState int32
type ChannelID int32

//some global variables
const (
	CHAN_MAX = 100 // the maximum number of channels is 100

)

const (
	PreAccepted   InstState = 0
	PreAcceptedOK InstState = 4
	Accepted      InstState = 1
	Committed     InstState = 2
	Prepare       InstState = 3
	Idle          InstState = 5
)

type ChangeStateMsg struct {
	success bool
}

type StatefulInst struct {
	inst  common.Instance
	state InstState
}

type InstList struct {
	Mu      sync.Mutex
	LogFile *os.File
	Offset  common.InstanceID
	Pending []*StatefulInst // one per InstanceID
}

// the state machine for each instance
type InstanceState struct {
	self int
	// channels for state transitions
	getReq         chan bool
	getPreAcceptOK chan bool
	selectFastPath chan bool
	getAcceptOK    chan bool

	state InstState
}

type EPaxos struct {
	self     common.ReplicaID
	lastInst common.InstanceID
	array    []*InstList // one InstList per replica
	data     map[common.Key]common.Value
	inbound  *chan interface{}
	udp      *net.UDPConn
	rpc      []chan interface{}
	peers    int // number of peers, including itself

	// records which channel is allocated for each instance
	inst2Chan map[common.InstanceID]ChannelID
	chanHead  int
	chanTail  int

	// channels for Instance state machines
	innerChan [CHAN_MAX]chan interface{}

	// channels to other servers/replicas
	inbound chan interface{}
}

func NewEPaxos(nrep int64, rep common.ReplicaID, endpoint string, buff int64) *EPaxos {
	dir := common.GetEnv("EPAXOS_DATA_PREFIX", "./data/data-")
	ep := new(EPaxos)
	ep.self = rep
	ep.array = make([]*InstList, nrep)
	ep.rpc = make([]chan interface{}, nrep)
	for i := int64(0); i < nrep; i++ {
		fileName := dir + strconv.FormatInt(i, 10) + ".dat"
		file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Print(err)
			return nil
		}
		lst := &InstList{
			Mu:      sync.Mutex{},
			LogFile: file,
			Offset:  0,
			Pending: make([]*StatefulInst, 0),
		}
		ep.array[i] = lst
		ep.rpc[i] = make(chan interface{}, buff)
	}
	ep.inbound = &ep.rpc[ep.self]
	addr, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		log.Println(err)
		return nil
	}
	log.Printf("ListenUDP on %s\n", endpoint)
	ep.udp, err = net.ListenUDP("udp", addr)
	if err != nil {
		log.Println(err)
		return nil
	}
	err = ep.recoverFromLog()
	if err != nil {
		log.Println(err)
		return nil
	}
	return ep
}

func (ep *EPaxos) ReadyProbe(payload string, ret *string) error {
	log.Printf("EPaxos.ReadyProbe with %s\n", payload)
	*ret = fmt.Sprintf("I'm EPaxos #%d, I'm alive", ep.self)
	return nil
}

func (ep *EPaxos) SendProbe(target common.ReplicaID, ret *string) error {
	log.Printf("EPaxos.SendProbe to %d\n", target)
	if int(target) >= len(ep.rpc) {
		return errors.New("out of range")
	}
	if target == ep.self {
		*ret = fmt.Sprintf("I'm EPaxos #%d, I don't send message to myself", ep.self)
		return nil
	}
	ep.rpc[target] <- common.ProbeMsg{Replica: ep.self}
	*ret = fmt.Sprintf("I'm EPaxos #%d, I sent message to %d", ep.self, target)
	return nil
}

type logWriter struct {
	Id common.ReplicaID
}

func (writer *logWriter) Write(bytes []byte) (int, error) {
	return fmt.Printf(
		"%s #%d %s",
		time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		writer.Id,
		string(bytes),
	)
}

func main() {
	logW := new(logWriter)
	logW.Id = -1
	log.SetFlags(log.Lshortfile)
	log.SetOutput(logW)
	rep, err := strconv.ParseInt(common.GetEnv("EPAXOS_REPLICA_ID", "0"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	logW.Id = common.ReplicaID(rep)

	log.Printf("This is epaxos-server, version %s", VERSION)
	nrep, err := strconv.ParseInt(common.GetEnv("EPAXOS_NREPLICAS", "1"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	buff, err := strconv.ParseInt(common.GetEnv("EPAXOS_BUFFER", "1024"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	endpoint := common.GetEnv("EPAXOS_LISTEN", "0.0.0.0:23333")
	addr, err := net.ResolveTCPAddr("tcp", endpoint)
	if err != nil {
		log.Fatal(err)
	}

	clientIn, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	ep := NewEPaxos(nrep, common.ReplicaID(rep), endpoint, buff)
	if ep == nil {
		log.Fatal("EPaxos creation failed")
	}

	err = ep.forkUdp()
	if err != nil {
		log.Fatal(err)
	}

	rpc.Register(ep)
	rpc.Accept(clientIn)
}
