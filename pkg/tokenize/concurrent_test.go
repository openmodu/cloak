package tokenize

import (
	"reflect"
	"sync"
	"testing"
)

func TestConcurrentNormalization(t *testing.T) {
	tk := New(NewVocab([]string{"[UNK]", "[CLS]", "[SEP]", "cafe", "resume", "naive"}), DefaultConfig())
	want := []string{"[CLS]", "cafe", "resume", "naive", "[SEP]"}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if got := tk.Encode("Café résumé naïve", 512).Tokens; !reflect.DeepEqual(got, want) {
					t.Errorf("tokens=%v", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}
