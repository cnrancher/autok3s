package utils

import (
	"sync"
	"sync/atomic"
)

type FirstErrGroup struct {
	errChan  chan error
	wg       *sync.WaitGroup
	errCount int32
}

func (g *FirstErrGroup) FirstError() <-chan error {
	go func() {
		g.wg.Wait()
		close(g.errChan)
	}()
	return g.errChan
}

func (g *FirstErrGroup) Go(f func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		errCount := atomic.LoadInt32(&g.errCount)
		if errCount > 0 {
			return
		}
		if err := f(); err != nil {
			g.sendError(err)
		}
	}()
}

func (g *FirstErrGroup) sendError(err error) {
	if err == nil {
		return
	}
	select {
	case g.errChan <- err:
	default:
	}
	atomic.AddInt32(&g.errCount, 1)
}

func (g *FirstErrGroup) Wait() int32 {
	g.wg.Wait()
	return g.errCount
}

func NewFirstErrorGroup() *FirstErrGroup {
	return &FirstErrGroup{
		wg:       &sync.WaitGroup{},
		errCount: 0,
		errChan:  make(chan error, 1),
	}
}
