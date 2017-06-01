package kafka

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		function func(*testing.T, context.Context, *Reader)
	}{
		{
			scenario: "calling Read with a context that has been canceled should return an error",
			function: testReaderReadCanceled,
		},

		{
			scenario: "all messages of the stream should be made available when calling ReadMessage repeatedly",
			function: testReaderReadMessages,
		},

		{
			scenario: "setting the offset to an invalid value should return an error on the next Read call",
			function: testReaderSetInvalidOffset,
		},

		{
			scenario: "setting the offset to random values should return the expected messages when Read is called",
			function: testReaderSetRandomOffset,
		},
	}

	for _, test := range tests {
		testFunc := test.function
		t.Run(test.scenario, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			r := NewReader(ReaderConfig{
				Brokers: []string{"localhost:9092"},
				Topic:   makeTopic(),
				MaxWait: 500 * time.Millisecond,
			})
			defer r.Close()
			testFunc(t, ctx, r)
		})
	}
}

func testReaderReadCanceled(t *testing.T, ctx context.Context, r *Reader) {
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	if _, err := r.ReadMessage(ctx); err != context.Canceled {
		t.Error(err)
	}
}

func testReaderReadMessages(t *testing.T, ctx context.Context, r *Reader) {
	const N = 1000
	prepareReader(t, ctx, r, makeTestSequence(N)...)

	for i := 0; i != N; i++ {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		v, _ := strconv.Atoi(string(m.Value))
		if v != i {
			t.Error("message at index", i, "has wrong value:", v)
			return
		}
	}
}

func testReaderSetInvalidOffset(t *testing.T, ctx context.Context, r *Reader) {
	r.SetOffset(42)

	_, err := r.ReadMessage(ctx)
	if err == nil {
		t.Error(err)
	}
}

func testReaderSetRandomOffset(t *testing.T, ctx context.Context, r *Reader) {
	const N = 10
	prepareReader(t, ctx, r, makeTestSequence(N)...)

	for i := 0; i != 2*N; i++ {
		offset := rand.Intn(N)
		r.SetOffset(int64(offset))
		m, err := r.ReadMessage(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		v, _ := strconv.Atoi(string(m.Value))
		if v != offset {
			t.Error("message at offset", offset, "has wrong value:", v)
			return
		}
	}
}

func makeTestSequence(n int) []Message {
	msgs := make([]Message, n)
	for i := 0; i != n; i++ {
		msgs[i] = Message{
			Value: []byte(strconv.Itoa(i)),
		}
	}
	return msgs
}

func prepareReader(t *testing.T, ctx context.Context, r *Reader, msgs ...Message) {
	config := r.Config()
	conn, err := DialLeader(ctx, "tcp", "localhost:9092", config.Topic, config.Partition)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.WriteMessages(msgs...); err != nil {
		t.Fatal(err)
	}
}