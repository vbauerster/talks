// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build OMIT

package main

import (
	"container/heap"
	"flag"
	"fmt"
	"math/rand"
	"time"
)

const nRequester = 100
const nWorker = 10

var roundRobin = flag.Bool("r", false, "use round-robin scheduling")

// Simulation of some work: just sleep for a while and report how long.
func op() int {
	n := rand.Int63n(int64(time.Second))
	time.Sleep(time.Duration(nWorker * n))
	return int(n)
}

type Request struct {
	fn func() int
	c  chan int
}

func requester(work chan Request) {
	c := make(chan int)
	for {
		time.Sleep(time.Duration(rand.Int63n(int64(nWorker * 2 * time.Second))))
		work <- Request{op, c}
		<-c
	}
}

type Worker struct {
	i        int
	requests chan Request
	pending  int
}

func (w *Worker) work(done chan *Worker) {
	for {
		req := <-w.requests
		req.c <- req.fn()
		done <- w
	}
}

type Pool []*Worker

func (p Pool) Len() int { return len(p) }

func (p Pool) Less(i, j int) bool {
	return p[i].pending < p[j].pending
}

func (p Pool) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
	p[i].i = i
	p[j].i = j
}

func (p *Pool) Push(x interface{}) {
	w := x.(*Worker)
	w.i = len(*p)
	// fmt.Printf("w = %+v\n", w)
	*p = append(*p, w)
}

func (p *Pool) Pop() interface{} {
	a := *p
	n := len(a)
	*p = a[0 : n-1]
	w := a[n-1]
	w.i = -1 // for safety
	return w
}

type Balancer struct {
	pool Pool
	done chan *Worker
	i    int
}

func NewBalancer() *Balancer {
	done := make(chan *Worker, nWorker)
	b := &Balancer{make(Pool, 0, nWorker), done, 0}
	for i := 0; i < nWorker; i++ {
		w := &Worker{requests: make(chan Request, nRequester)}
		heap.Push(&b.pool, w)
		go w.work(b.done)
	}
	return b
}

func (b *Balancer) balance(work chan Request) {
	for {
		select {
		case req := <-work:
			b.dispatch(req)
		case w := <-b.done:
			b.completed(w)
		}
		b.print()
	}
}

func (b *Balancer) print() {
	sum := 0
	sumsq := 0
	for _, w := range b.pool {
		fmt.Printf("%d ", w.pending)
		sum += w.pending
		sumsq += w.pending * w.pending
	}
	avg := float64(sum) / float64(len(b.pool))
	variance := float64(sumsq)/float64(len(b.pool)) - avg*avg
	fmt.Printf(" %.2f %.2f\n", avg, variance)
}

func (b *Balancer) dispatch(req Request) {
	if *roundRobin {
		w := b.pool[b.i]
		w.requests <- req
		w.pending++
		b.i++
		if b.i >= len(b.pool) {
			b.i = 0
		}
		return
	}

	w := heap.Pop(&b.pool).(*Worker)
	w.requests <- req
	w.pending++
	// fmt.Printf("started %p; now %d\n", w, w.pending)
	heap.Push(&b.pool, w)
}

func (b *Balancer) completed(w *Worker) {
	if *roundRobin {
		w.pending--
		return
	}

	w.pending--
	// fmt.Printf("finished %p; now %d\n", w, w.pending)
	// heap.Remove(&b.pool, w.i)
	// heap.Push(&b.pool, w)
	heap.Fix(&b.pool, w.i)
}

func main() {
	flag.Parse()
	work := make(chan Request)
	for i := 0; i < nRequester; i++ {
		go requester(work)
	}
	NewBalancer().balance(work)
}
