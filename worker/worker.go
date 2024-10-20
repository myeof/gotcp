package worker

import (
	"log"
	"sync"
	"sync/atomic"
)

type Worker struct {
	wg   sync.WaitGroup
	jobs chan func()
	n    int64
}

// NewWorker create a new worker
// n: number of workers
// tql: task queue length
func NewWorker(n, l int) *Worker {
	w := Worker{
		jobs: make(chan func(), l),
	}
	// 启动多个goroutine作为worker
	for i := 0; i < n; i++ {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			for {
				job, ok := <-w.jobs
				if !ok {
					return
				}
				atomic.AddInt64(&w.n, 1)
				job()
				atomic.AddInt64(&w.n, -1)
			}
		}()
	}
	return &w
}

func (w *Worker) StartJob(job func()) {
	w.jobs <- job
}

func (w *Worker) Shutdown() chan struct{} {
	close(w.jobs)
	w.wg.Wait()
	ch := make(chan struct{})
	defer close(ch)
	return ch
}

func (w *Worker) Status() {
	log.Printf("[%d] jobs running, [%d] job pendding", w.n, len(w.jobs))
}
