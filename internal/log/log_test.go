package log

import (
	"io/ioutil"
	"os"
	"testing"

	api "github.com/angelarvm/prolog/api/v1"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, log *Log,
	){
		"append and read a record succeeds": testAppendRead,
		"offset out of range error":         testOutOfRangeErr,
		"init with existing segments":       testInitExisting,
		"reader":                            testReader,
		"truncate":                          testTruncate,
	} {
		t.Run(scenario, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "store-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			c := Config{}
			c.Segment.MaxIndexBytes = 32
			log, err := NewLog(dir, c)
			require.NoError(t, err)

			fn(t, log)
		})
	}
}

func testAppendRead(t *testing.T, l *Log) {
	append := &api.Record{
		Value: []byte("hello world"),
	}

	off, err := l.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	read, err := l.Read(off)
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testOutOfRangeErr(t *testing.T, l *Log) {
	read, err := l.Read(1)
	require.Nil(t, read)
	apiErr := err.(api.ErrOffsetOutOfRange)
	require.Equal(t, err, apiErr)
}

func testInitExisting(t *testing.T, l *Log) {
	append := &api.Record{
		Value: []byte("hello world"),
	}

	for range 3 {
		_, err := l.Append(append)
		require.NoError(t, err)
	}
	require.NoError(t, l.Close())

	off, err := l.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	off, err = l.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), off)

	n, err := NewLog(l.Dir, l.Config)
	require.NoError(t, err)

	off, err = n.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	off, err = n.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), off)
}

func testReader(t *testing.T, l *Log) {
	append := &api.Record{
		Value: []byte("hello world"),
	}
	off, err := l.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	reader := l.Reader()
	b, err := ioutil.ReadAll(reader)
	require.NoError(t, err)

	read := &api.Record{}
	err = proto.Unmarshal(b[lenWitdh:], read)
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testTruncate(t *testing.T, l *Log) {
	append := &api.Record{
		Value: []byte("hello world"),
	}

	for range 3 {
		_, err := l.Append(append)
		require.NoError(t, err)
	}

	err := l.Truncate(1)
	require.NoError(t, err)

	_, err = l.Read(0)
	require.Error(t, err)
}
