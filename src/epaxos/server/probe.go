package main

import "log"
import "math/rand"
import "epaxos/common"

func (ep *EPaxos) allocProbe() (int64, chan bool) {
	ep.probesL.Lock()
	defer ep.probesL.Unlock()
	var probeId int64
	for {
		probeId = rand.Int63()
		if _, ok := ep.probes[probeId]; !ok {
			break
		}
	}
	ch := make(chan bool)
	log.Printf("ProbeId %d allocated:", probeId)
	ep.probes[probeId] = ch
	return probeId, ch
}

func (ep *EPaxos) freeProbe(probeId int64) {
	ep.probesL.Lock()
	defer ep.probesL.Unlock()
	log.Printf("ProbeId %d freed:", probeId)
	delete(ep.probes, probeId)
}

func (ep *EPaxos) recvProbe(m *common.ProbeMsg) {
	log.Print(m)
	if m.RequestReply {
		log.Printf("Received ProbeMsg from %d, will reply\n", m.Replica)
		ep.rpc[m.Replica] <- common.ProbeMsg{
			Replica:      ep.self,
			Payload:      m.Payload,
			RequestReply: false,
		}
	} else {
		log.Printf("Received ProbeMsg from %d\n", m.Replica)
		ep.probesL.Lock()
		defer ep.probesL.Unlock()
		log.Printf("ProbeId %d received:", m.Payload)
		if ch, ok := ep.probes[m.Payload]; ok {
			ch <- true
		}
	}
}
