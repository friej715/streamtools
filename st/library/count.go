package library

import (
	"container/heap"
	"github.com/nytlabs/streamtools/st/blocks" // blocks
	"github.com/nytlabs/streamtools/st/util"
	"time"
)

type Count struct {
	blocks.Block
	queryrule  chan chan interface{}
	querycount chan chan interface{}
	inrule     chan interface{}
	inpoll     chan interface{}
	clear      chan interface{}
	in         chan interface{}
	out        chan interface{}
	quit       chan interface{}
}

// a bit of boilerplate for streamtools
func NewCount() blocks.BlockInterface {
	return &Count{}
}

func (b *Count) Setup() {
	b.Kind = "Count"
	b.Desc = "counts the number of messages it has seen over the specified Window"
	b.in = b.InRoute("in")
	b.inrule = b.InRoute("rule")
	b.inpoll = b.InRoute("poll")
	b.clear = b.InRoute("clear")
	b.queryrule = b.QueryRoute("rule")
	b.querycount = b.QueryRoute("count")
	b.quit = b.Quit()
	b.out = b.Broadcast()
}

func (b *Count) Run() {
	waitTimer := time.NewTimer(100 * time.Millisecond)
	pq := &PriorityQueue{}
	heap.Init(pq)
	window := time.Duration(0)

	for {
		select {
		case <-waitTimer.C:
		case rule := <-b.inrule:

			tmpDurStr, err := util.ParseString(rule, "Window")
			if err != nil {
				b.Error(err)
				continue
			}

			tmpWindow, err := time.ParseDuration(tmpDurStr)
			if err != nil {
				b.Error(err)
				continue
			}

			window = tmpWindow
		case <-b.quit:
			return
		case <-b.in:
			empty := make([]byte, 0)
			queueMessage := &PQMessage{
				val: &empty,
				t:   time.Now(),
			}
			heap.Push(pq, queueMessage)
		case <-b.clear:
			for len(*pq) > 0 {
				heap.Pop(pq)
			}
		case <-b.inpoll:
			b.out <- map[string]interface{}{
				"Count": float64(len(*pq)),
			}
		case c := <-b.queryrule:
			c <- map[string]interface{}{
				"Window": window.String(),
			}
		case c := <-b.querycount:
			c <- map[string]interface{}{
				"Count": float64(len(*pq)),
			}
		}
		for {
			pqMsg, diff := pq.PeekAndShift(time.Now(), window)
			if pqMsg == nil {
				// either the queue is empty, or it"s not time to emit
				if diff == 0 {
					// then the queue is empty. Pause for 5 seconds before checking again
					diff = time.Duration(500) * time.Millisecond
				}
				waitTimer.Reset(diff)
				break
			}
		}
	}
}
