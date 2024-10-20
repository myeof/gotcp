package worker

// go test -v onemt-file/p2p/worker
// go test -v onemt-file/p2p/worker -run TestInitWorker
import (
	"testing"
	"time"
)

func TestInitWorker(t *testing.T) {
	w := NewWorker(4, 4)
	if w == nil {
		t.Fatal("Worker is nil")
	}
	w.StartJob(func() {
		t.Log("Worker1 running")
	})
	w.StartJob(func() {
		t.Log("Worker2 running")
		time.Sleep(100 * time.Millisecond)
	})
	<-w.Shutdown()
	t.Log("Worker shutdown")
}
