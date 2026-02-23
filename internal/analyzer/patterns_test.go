package analyzer

import (
	"testing"
)

func TestPatternMatch_GoGoroutines(t *testing.T) {
	diff := `
+func main() {
+    var wg sync.WaitGroup
+    wg.Add(1)
+    go func() {
+        defer wg.Done()
+        doWork()
+    }()
+    wg.Wait()
+}
`
	matches := PatternMatch(diff)
	if len(matches) == 0 {
		t.Fatal("expected at least 1 match")
	}
	found := false
	for _, m := range matches {
		if m.TopicSlug == "go-goroutines" {
			found = true
			if m.Confidence != 0.85 {
				t.Errorf("confidence = %f, want 0.85", m.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected go-goroutines match")
	}
}

func TestPatternMatch_GoChannels(t *testing.T) {
	diff := `
+ch := make(chan int, 10)
+ch <- 42
+val := <-ch
`
	matches := PatternMatch(diff)
	found := false
	for _, m := range matches {
		if m.TopicSlug == "go-channels" {
			found = true
		}
	}
	if !found {
		t.Error("expected go-channels match")
	}
}

func TestPatternMatch_ReactState(t *testing.T) {
	diff := `
+const [count, setCount] = useState(0)
+const [name, setName] = useState("")
`
	matches := PatternMatch(diff)
	found := false
	for _, m := range matches {
		if m.TopicSlug == "react-state" {
			found = true
		}
	}
	if !found {
		t.Error("expected react-state match")
	}
}

func TestPatternMatch_ReactEffects(t *testing.T) {
	diff := `
+useEffect(() => {
+    fetchData(userId)
+    return () => cleanup()
+}, [userId])
`
	matches := PatternMatch(diff)
	found := false
	for _, m := range matches {
		if m.TopicSlug == "react-effects" {
			found = true
		}
	}
	if !found {
		t.Error("expected react-effects match")
	}
}

func TestPatternMatch_MultipleMatches(t *testing.T) {
	diff := `
+go func() {
+    ch := make(chan int)
+    select {
+    case v := <-ch:
+        fmt.Println(v)
+    case <-ctx.Done():
+        return
+    }
+}()
+context.WithCancel(context.Background())
`
	matches := PatternMatch(diff)
	if len(matches) < 3 {
		t.Errorf("got %d matches, want at least 3", len(matches))
	}
}

func TestPatternMatch_Empty(t *testing.T) {
	matches := PatternMatch("")
	if len(matches) != 0 {
		t.Errorf("got %d matches for empty diff, want 0", len(matches))
	}
}

func TestPatternMatch_Max5(t *testing.T) {
	// Diff that matches many patterns
	diff := `
+go func() {}()
+sync.WaitGroup{}
+make(chan int)
+select {
+case <-ch:
+}
+context.Background()
+sync.Mutex{}
+m.Lock()
+io.Reader
+fmt.Errorf("err: %w", err)
+errors.Is(err, io.EOF)
+useEffect(() => {}, [])
+useState(0)
+Promise.all([])
+async function foo() { await bar() }
`
	matches := PatternMatch(diff)
	if len(matches) > 5 {
		t.Errorf("got %d matches, want max 5", len(matches))
	}
}

func TestPatternMatch_JSPromises(t *testing.T) {
	diff := `
+const results = await Promise.all([fetchA(), fetchB()])
+const p = new Promise((resolve, reject) => {})
`
	matches := PatternMatch(diff)
	found := false
	for _, m := range matches {
		if m.TopicSlug == "js-promises" {
			found = true
		}
	}
	if !found {
		t.Error("expected js-promises match")
	}
}

func TestPatternMatch_JSAsyncAwait(t *testing.T) {
	diff := `
+async function fetchData() {
+    const data = await fetch(url)
+    return data.json()
+}
`
	matches := PatternMatch(diff)
	found := false
	for _, m := range matches {
		if m.TopicSlug == "js-async-await" {
			found = true
		}
	}
	if !found {
		t.Error("expected js-async-await match")
	}
}
