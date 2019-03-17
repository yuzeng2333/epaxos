package main

import "os"
import "sync"
import "log"
import "net"
import "net/rpc"
import "epaxos/common"

type InstState int32

const (
	PreAccepted InstState = 0
	Accepted    InstState = 1
	Committed   InstState = 2
	Prepare     InstState = 3
)

type StatefulInst struct {
	inst  common.Instance
	state InstState
}

type InstList struct {
	Mu      sync.Mutex
	LogFile *os.File
	Offset  common.InstanceID
	Pending []*StatefulInst
}

type EPaxos struct {
	self     common.ReplicaID
	lastInst common.InstanceID
	array    map[common.ReplicaID]*InstList
	data     map[common.Key]common.Value
}

func NewEPaxos(nrep int64, rep common.ReplicaID) *EPaxos {
	ep := new(EPaxos)
	ep.self = rep
	// TODO
	err := ep.recoverFromLog()
	return ep
}

func main() {
	addy, err := net.ResolveTCPAddr("tcp", "0.0.0.0:23333")
	if err != nil {
		log.Fatal(err)
	}

	inbound, err := net.ListenTCP("tcp", addy)
	if err != nil {
		log.Fatal(err)
	}

	ep := NewEPaxos(3, 0) // TODO
	rpc.Register(ep)
	rpc.Accept(inbound)
}
