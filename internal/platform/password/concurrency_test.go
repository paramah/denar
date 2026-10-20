package password

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Each derivation takes 64 MiB, so however many logins arrive at once, only
// maxConcurrent of them may be deriving a key.
func TestDerivationsAreBounded(t *testing.T) {
	var running, peak atomic.Int32
	original := deriveKey
	deriveKey = func(password, salt []byte, time_, memory uint32, threads uint8, keyLen uint32) []byte {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		running.Add(-1)
		return make([]byte, keyLen)
	}
	t.Cleanup(func() { deriveKey = original })

	h := Hasher{}
	hash, err := h.Hash(secret)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			if i%2 == 0 {
				h.Verify(secret, hash)
			} else {
				h.Hash(secret)
			}
		})
	}
	wg.Wait()

	if got := peak.Load(); got != maxConcurrent {
		t.Errorf("peak of concurrent derivations = %d, want %d", got, maxConcurrent)
	}
}

func TestMalformedHashTakesNoSlot(t *testing.T) {
	for range maxConcurrent {
		slots <- struct{}{}
	}
	t.Cleanup(func() {
		for range maxConcurrent {
			<-slots
		}
	})

	done := make(chan bool)
	go func() { done <- Hasher{}.Verify(secret, "not a hash") }()
	select {
	case ok := <-done:
		if ok {
			t.Error("Verify() = true for a malformed hash")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Verify of a malformed hash waited for a derivation slot")
	}
}
