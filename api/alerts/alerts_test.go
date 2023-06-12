package alerts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-json-experiment/json"
)

func TestAlertSubFan(t *testing.T) {
	if os.Getenv("REDIS_ADDRESS") == "" {
		t.Skip("skipping test; $REDIS_ADDRESS not set")
	}

	s, err := NewStore([]string{os.Getenv("REDIS_ADDRESS")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	type msg struct {
		Msg  string
		Hash []byte
	}

	count := atomic.Int32{}
	sendCount := 6
	subCount := 12
	expectCount := sendCount * subCount
	// send 6 messages

	fn := func(name string) (string, func(ctx context.Context, message msg)) {
		return name, func(_ context.Context, message msg) {
			t.Logf("received message: {msg: %+v, hash: %+x} from %s", message.Msg, message.Hash, name)

			decode, err := base64.StdEncoding.DecodeString(message.Msg)
			if err != nil {
				t.Fatalf("failed to decode message: %v", err)
			}

			if string(sha256.New().Sum(decode)) != string(message.Hash) {
				t.Errorf("hashes do not match")
				return
			}

			count.Add(1)
		}
	}

	subFan := SubFan[msg](context.Background())
	for i := 0; i < subCount; i++ {
		subFan.Subscribe(fn(fmt.Sprintf("subscriber %d", i)))
	}

	go s.AlertSub(context.Background(), subFan)

	for i := 0; i <= sendCount; i++ {
		buf := make([]byte, 32)
		rand.Read(buf)

		m := msg{
			Msg:  base64.StdEncoding.EncodeToString(buf),
			Hash: sha256.New().Sum(buf),
		}

		j, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("failed to marshal message: %v", err)
		}

		err = s.client.Do(
			context.Background(),
			s.client.B().Publish().Channel("alerts").Message(string(j)).Build(),
		).Error()
		if err != nil {
			t.Fatalf("failed to publish message: %v", err)
		}
	}

	start := time.Now()

	for ; start.Add(time.Second * 5).After(time.Now()); time.Sleep(100 * time.Millisecond) {
		if int(count.Load()) == expectCount {
			t.Logf("received all messages, unsubscribing")
			subFan.Unsubscribe("test")
			subFan.Unsubscribe("test2")
			return
		}
	}

	t.Errorf("expected %d messages, got %d", expectCount, count.Load())
}
