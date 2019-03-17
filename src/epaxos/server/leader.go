package main

import "os"
import "sync"
import "log"
import "net"
import "net/rpc"
import "epaxos/common"
import "math"

// EPaxos only distributes msgs from client and to Instance 
// state machines. However, the state machines will send messages
// to other replicas directly, without going through the EPaxos

type instStateMachine struct {
    self            int
    state           InstState

    // preAccept phase
    FQuorum         []ReplicaID
    preAcceptNo     int // number for received preAccept msg
    preAcceptBook   map[ReplicaID]bool // record from which preAccept is received

    // preAcceptOK phase
    chooseFast      bool
    seqOK           Sequence
    depsOK          []InstRef
}


// func to make a new server
func Make(me int, inbound interface) *EPaxos {
    ep := &EPaxos{}
    ep.self = me
    ep.lastInst = common.InstanceID(0)
    ep.array = //TODO
    ep.data = make(map[common.Key]common.Value)
    ep.inst2Chan = make(map[common.InstanceID]ChannelID)
    ep.chanHead = 0
    ep.chanTail = 0

    ep.innerChan = make(chan interface{}, CHAN_MAX)
    ep.inbound = inbound
    ep.rpc = // TODO

    go ep.startServer()

    return ep
}

func (ep *EPaxos) startServer() {
    var instId common.InstanceID
    for {
        msg := <-ep.inbound
        switch msg.(type) {
            case RequestMsg:
                //TODO: write into array
                // If no free channel, report error
                if ep.chanHead == ep.chanTail + 1 {
                    log.Fatal("No free channel for new instance machine")
                }
                ep.inst2Chan[++ep.lastInst] = ep.chanTail++
                go ep.startInstanceState(ep.lastInst)

            case PreAccepOKtMsg:
                instId = msg.Id.Inst
                ep.innerChan[instId] <- msg

            case AcceptOKMsg:
                instId = msg.Id.Inst
                ep.innerChan[instId] <- msg
        }
    }
}

func interfCmd(cmd1 Command, cmd2 Command) bool {
    return cmd1.Key == cmd2.Key
           && ( (cmd1.CmdType == CmdPut && cmd2.CmdType == CmdNoOp)
           || (cmd1.CmdType == CmdNoOp && cmd2.CmdType == CmdPut)
           || (cmd1.CmdType == CmdPut && cmd2.CmdType == CmdPut
           && cmd1.Value != cmd2.Value) )
}

// TODO: make a more fast implementation
func compareMerge(dep1 *[]InstRef, dep2 []InstRef) bool { // return true if the same
    ret := true
    for index2, id2 := range dep2 {
        exist := false
        for index1, id1 := range *dep1 {
            if id1 == id2 {
                exist = true
                break
            }
        }
        if exist == false {
            ret = false
            *dep1 = append(*dep1, id2)
        }
    }
    if ret == false {
        return ret
    }
    if len(*dep1) > len(dep2) {
        return false
    }
    return ret
}

func (ep *EPaxos) startInstanceState(instId int, cmd Command) {
    // ism abbreviated to instance state machine
    ism := &instStateMachine{}
    ism.self = instId
    ism.state = Idle // starting from idle state, send preaccept to F
    ism.preAcceptNo = 0
    ism.chooseFast = true
    var innerMsg interface{}

    for {
        switch ism.state {
        case Idle:
            deps := []InstRef
            seqMax := 0
            for index1, oneReplica := range ep.array {
                for index2, oneInst := range oneReplica {
                    if interfCmd(oneInst.inst.Cmd, cmd) {
                        deps = append(deps, InstRef{Replica: index1, Inst: index2})
                        if oneInst.inst.Seq > seqMax {
                            seqMax = oneInst.inst.Seq
                        }
                    }
                }
            }
            seq := seqMax + 1
            inst := &Instance{}
            inst.Cmd = cmd
            inst.Seq = seq
            inst.Deps = deps
            ism.seqOK = seq
            ism.depOK = deps
            ep.array[ep.self].Pending[instId] = StatefulInst{inst: inst, state:PreAccepted}

            // send PreAccept to all other replicas in F
            F := math.floor(ep.peers / 2)
            sendMsg := &PreAcceptMsg{}
            sendMsg.Inst = inst
            sendMsg.Id = InstRef{ep.self, instId}
            sim.Fquorum = Fep.makeMulticast(sendMsg, F-1)
            ism.preAcceptNo = F-1
            ism.state = PreAccepted


        case PreAccepted:
            select {
                case innerMsg = <-ep.innerChan:
                    if innerMsg.(type) == PreAccepOKMsg {
                        // check instId is correct
                        if innerMsg.Id.Inst != ism.self {
                            log.Fatal("Wrong inner msg!")
                        }
                        // if the msg has been received from the sender, break
                        if ism.preAcceptBook[innerMsg.sender] == true {
                            break;
                        }
                        ism.preAcceptBook[innerMsg.sender] = true
                        ism.preAcceptNo--
                        // compare seq and dep
                        if innerMsg.Seq > ism.seqOK {
                            ism.chooseFast = false
                            ism.seqOK = innerMsg.Seq
                        }
                        if compareMerge(ism.depOK, innerMsg.Deps) == false {
                            ism.chooseFast = false
                        }
                        if ism.preAcceptNo == 0 {
                            if ism.chooseFast == true {
                                ism.state = Committed
                            }
                            else {
                                ism.state = Accepted
                            }
                        }
                    }
                default:
            }

        case Accepted:
            inst := &Instance{}
            inst.Cmd = cmd
            inst.Seq = seqOK
            inst.Deps = depOK
            ep.array[ep.self].Pending[instId] = StatefulInst{inst: inst, state:Accepted}
            

        case LeaderCommit:


        }
    }
}
