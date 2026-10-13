package redact

import (
	"bytes"
	"encoding/base64"
	"errors"
	"math/rand/v2"
	"net/url"
	"sort"
	"strings"
	"testing"
)

// reference computes leftmost-longest non-overlapping replacement using
// strings.NewReplacer with secrets ordered longest first.
func reference(input string, secrets []string) string {
	var olds []string
	seen := map[string]bool{}
	for _, s := range secrets {
		if s != "" && !seen[s] {
			seen[s] = true
			olds = append(olds, s)
		}
	}
	if len(olds) == 0 {
		return input
	}
	sort.SliceStable(olds, func(i, j int) bool { return len(olds[i]) > len(olds[j]) })
	var args []string
	for _, o := range olds {
		args = append(args, o, Placeholder)
	}
	return strings.NewReplacer(args...).Replace(input)
}

func toBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// run writes input in chunks of the given sizes (cycled) and returns output.
func run(t *testing.T, input string, secrets []string, chunks []int) string {
	t.Helper()
	var out bytes.Buffer
	w := NewWriter(&out, toBytes(secrets))
	maxLen := 0
	for _, s := range secrets {
		maxLen = max(maxLen, len(s))
	}
	data := []byte(input)
	ci := 0
	for len(data) > 0 {
		n := len(data)
		if len(chunks) > 0 {
			n = min(chunks[ci%len(chunks)], len(data))
			ci++
		}
		got, err := w.Write(data[:n])
		if err != nil || got != n {
			t.Fatalf("Write = %d, %v; want %d, nil", got, err, n)
		}
		data = data[n:]
		if maxLen > 0 && len(w.buf) > maxLen-1 {
			t.Fatalf("held back %d bytes, max allowed %d", len(w.buf), maxLen-1)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return out.String()
}

func TestWriterTable(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		input   string
		want    string
	}{
		{"no secrets", nil, "hello", "hello"},
		{"empty secret ignored", []string{""}, "hello", "hello"},
		{"simple", []string{"s3cr3t"}, "a s3cr3t b", "a [REDACTED] b"},
		{"repeated", []string{"ab"}, "ababab", "[REDACTED][REDACTED][REDACTED]"},
		{"at end", []string{"tail"}, "the tail", "the [REDACTED]"},
		{"partial at end", []string{"tailx"}, "the tail", "the tail"},
		{"longest wins", []string{"abc", "abcdef"}, "xabcdefy", "x[REDACTED]y"},
		{"shorter when longer fails", []string{"abc", "abcdef"}, "xabcdeZ", "x[REDACTED]deZ"},
		{"overlap leftmost", []string{"abcd", "cdef"}, "abcdef", "[REDACTED]ef"},
		{"overlap second", []string{"bcd", "abx"}, "abcd", "a[REDACTED]"},
		{"nested", []string{"bc", "abcd"}, "abcabcd", "a[REDACTED][REDACTED]"},
		{"self overlap", []string{"aaa"}, "aaaaaaa", "[REDACTED][REDACTED]a"},
		{"prefix restart", []string{"aab"}, "aaab", "a[REDACTED]"},
		{"single byte secret", []string{"x"}, "axbxc", "a[REDACTED]b[REDACTED]c"},
		{"duplicates", []string{"k", "k"}, "kk", "[REDACTED][REDACTED]"},
		{"binary", []string{"\x00\xff"}, "a\x00\xffb", "a[REDACTED]b"},
		{"placeholder-like", []string{"RED"}, "[REDACTED]", "[[REDACTED]ACTED]"},
	}
	chunkings := [][]int{nil, {1}, {2}, {3}, {1, 2, 5}, {7}}
	for _, tt := range tests {
		for _, ch := range chunkings {
			got := run(t, tt.input, tt.secrets, ch)
			if got != tt.want {
				t.Errorf("%s chunks=%v: got %q want %q", tt.name, ch, got, tt.want)
			}
			if ref := reference(tt.input, tt.secrets); ref != tt.want {
				t.Errorf("%s: reference %q disagrees with want %q", tt.name, ref, tt.want)
			}
		}
	}
}

func TestSingleByteWrites(t *testing.T) {
	secrets := []string{"correct-horse-battery-staple", "hunter2", "battery"}
	input := strings.Repeat("prefix correct-horse-battery-staple mid hunter2 battery hunter correct-horse-batt ", 20)
	got := run(t, input, secrets, []int{1})
	want := reference(input, secrets)
	if got != want {
		t.Fatalf("single-byte writes mismatch")
	}
	for _, s := range []string{"hunter2", "correct-horse-battery-staple"} {
		if strings.Contains(got, s) {
			t.Fatalf("secret leaked")
		}
	}
}

func TestOutputIsStreamed(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, [][]byte{[]byte("secret")})
	if _, err := w.Write([]byte("hello world sec")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello world " {
		t.Fatalf("streamed %q", got)
	}
	if _, err := w.Write([]byte("ret!")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello world [REDACTED]!" {
		t.Fatalf("got %q", got)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal("second close", err)
	}
	if _, err := w.Write([]byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("write after close: %v", err)
	}
}

func TestCloseFlushesPartial(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, [][]byte{[]byte("abcdef"), []byte("abc")})
	w.Write([]byte("xxabcde"))
	if out.String() != "xx" {
		t.Fatalf("got %q", out.String())
	}
	w.Close()
	if out.String() != "xx[REDACTED]de" {
		t.Fatalf("got %q", out.String())
	}
}

type failWriter struct{}

var errFail = errors.New("boom")

func (failWriter) Write(p []byte) (int, error) { return 0, errFail }

func TestUnderlyingError(t *testing.T) {
	w := NewWriter(failWriter{}, [][]byte{[]byte("zz")})
	if _, err := w.Write([]byte("hello")); !errors.Is(err, errFail) {
		t.Fatalf("got %v", err)
	}
	if _, err := w.Write([]byte("hello")); !errors.Is(err, errFail) {
		t.Fatalf("sticky: got %v", err)
	}
	if err := w.Close(); !errors.Is(err, errFail) {
		t.Fatalf("close: got %v", err)
	}
}

func TestRandomizedAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	alphabet := "abc"
	randStr := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[rng.IntN(len(alphabet))]
		}
		return string(b)
	}
	for iter := 0; iter < 3000; iter++ {
		ns := 1 + rng.IntN(4)
		var secrets []string
		for range ns {
			secrets = append(secrets, randStr(1+rng.IntN(6)))
		}
		// Plant secrets in the input often.
		var sb strings.Builder
		for sb.Len() < 1+rng.IntN(200) {
			if rng.IntN(3) == 0 {
				sb.WriteString(secrets[rng.IntN(len(secrets))])
			} else {
				sb.WriteString(randStr(1 + rng.IntN(4)))
			}
		}
		input := sb.String()
		var chunks []int
		for range 1 + rng.IntN(5) {
			chunks = append(chunks, 1+rng.IntN(8))
		}
		got := run(t, input, secrets, chunks)
		want := reference(input, secrets)
		if got != want {
			t.Fatalf("iter %d: secrets=%q input=%q chunks=%v\n got %q\nwant %q", iter, secrets, input, chunks, got, want)
		}
	}
}

func TestVariants(t *testing.T) {
	if Variants("") != nil {
		t.Fatal("empty secret should yield nil")
	}
	s := "p@ss w/rd+\"<x>\"?\n\xff"
	v := Variants(s)
	want := []string{
		s,
		base64.StdEncoding.EncodeToString([]byte(s)),
		base64.URLEncoding.EncodeToString([]byte(s)),
		base64.RawURLEncoding.EncodeToString([]byte(s)),
		url.QueryEscape(s),
		`p@ss w/rd+\"\u003cx\u003e\"?\n\ufffd`,
		`p@ss w/rd+\"<x>\"?\n\ufffd`,
	}
	set := map[string]bool{}
	for _, x := range v {
		if x == "" {
			t.Fatal("empty variant")
		}
		if set[x] {
			t.Fatal("duplicate variant")
		}
		set[x] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing variant %q", w)
		}
	}
	// Simple ASCII secret: base64 forms coincide, no duplicates.
	v = Variants("abcdef")
	if v[0] != "abcdef" {
		t.Fatalf("raw first: %q", v)
	}
	if len(v) != 2 { // raw, base64 "YWJjZGVm" (all other encodings identical)
		t.Fatalf("variants %q", v)
	}
}

func TestVariantsRedactInStream(t *testing.T) {
	s := "tok/en+=?&"
	var secrets [][]byte
	for _, v := range Variants(s) {
		secrets = append(secrets, []byte(v))
	}
	var out bytes.Buffer
	w := NewWriter(&out, secrets)
	input := "raw=" + s + " q=" + url.QueryEscape(s) + " b=" + base64.StdEncoding.EncodeToString([]byte(s)) +
		" u=" + base64.RawURLEncoding.EncodeToString([]byte(s))
	for i := range len(input) {
		w.Write([]byte{input[i]})
	}
	w.Close()
	got := out.String()
	if got != "raw=[REDACTED] q=[REDACTED] b=[REDACTED] u=[REDACTED]" {
		t.Fatalf("got %q", got)
	}
}

func BenchmarkWriter(b *testing.B) {
	secrets := [][]byte{}
	for i := range 200 {
		secrets = append(secrets, []byte("secret-value-"+strings.Repeat("x", i%10)+string(rune('a'+i%26))))
	}
	data := bytes.Repeat([]byte("log line with some secret-value text and other output\n"), 1000)
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		w := NewWriter(ioDiscard{}, secrets)
		w.Write(data)
		w.Close()
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
