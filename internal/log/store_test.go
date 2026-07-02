package log

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	write = []byte("hello world")
	width = uint64(len(write)) + lenWitdh
)

func TestStoreAppendRead(t *testing.T) {
	f, err := ioutil.TempFile("", "store_append_read_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	s, err := newStore(f)
	require.NoError(t, err)

	testAppend(t, s)
	testRead(t, s)
	testReadAt(t, s)

	s, err = newStore(f)
	require.NoError(t, err)
	testRead(t, s)
}

func testAppend(t *testing.T, s *store) {
	t.Helper()
	for i := uint64(1); i < 4; i++ {
		n, pos, err := s.Append(write)
		require.NoError(t, err)
		require.Equal(t, pos+n, width*i)
	}
}

func testRead(t *testing.T, s *store) {
	t.Helper()
	var pos uint64
	for i := uint64(1); i < 4; i++ {
		read, err := s.Read(pos)
		require.NoError(t, err)
		require.Equal(t, write, read)
		pos += width
	}
}

func testReadAt(t *testing.T, s *store) {
	t.Helper()
	for i, off := uint64(1), int64(0); i < 4; i++ {
		b := make([]byte, lenWitdh)
		n, err := s.ReadAt(b, off)
		require.NoError(t, err)
		require.Equal(t, lenWitdh, n)
		off += int64(n)

		size := enc.Uint64(b)
		b = make([]byte, size)
		n, err = s.ReadAt(b, off)
		require.NoError(t, err)
		require.Equal(t, write, b)
		require.Equal(t, int(size), n)
		off += int64(n)
	}
}

func TestStoreClose(t *testing.T) {
	file, err := ioutil.TempFile("", "store_close_test")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	store, err := newStore(file)
	require.NoError(t, err)
	_, _, err = store.Append(write)
	require.NoError(t, err)

	file, beforeSize, err := openFile(file.Name())
	require.NoError(t, err)
	defer file.Close()

	err = store.Close()
	require.NoError(t, err)

	file, afterSize, err := openFile(file.Name())
	require.NoError(t, err)
	defer file.Close()
	require.True(t, afterSize > beforeSize)
}

func openFile(name string) (f *os.File, size int64, err error) {
	file, err := os.OpenFile(
		name,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, 0, err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}

	return file, fi.Size(), nil
}
