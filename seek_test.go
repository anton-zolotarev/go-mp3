// Copyright 2026 The go-mp3 Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mp3_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hajimehoshi/go-mp3"
)

func decodeAll(t *testing.T, path string) (*mp3.Decoder, []byte) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	d, err := mp3.NewDecoder(f)
	if err != nil {
		t.Fatal(err)
	}
	all, err := ioutil.ReadAll(d)
	if err != nil {
		t.Fatal(err)
	}
	return d, all
}

// TestSeekMatchesReadingThrough: the samples read after a seek are those
// read at the same place going through the stream from the start — at,
// just after and inside a frame's start. A frame depends on those before
// it (its main data may begin in theirs, and it overlaps the last one), so
// a seek that does not decode enough frames before the place gives broken
// samples there: a click.
func TestSeekMatchesReadingThrough(t *testing.T) {
	for _, c := range []struct {
		path          string
		samplesPerFrm int64
	}{
		{"example/classic.mp3", 1152}, // MPEG-1 Layer III
		{"example/mpeg2.mp3", 576},    // MPEG-2
	} {
		t.Run(c.path, func(t *testing.T) {
			d, all := decodeAll(t, c.path)
			if int64(len(all)) != d.Length() {
				t.Fatalf("read %d bytes, Length %d", len(all), d.Length())
			}
			bpf := 4 * c.samplesPerFrm // bytes per frame of output
			var places []int64
			for _, f := range []int64{1, 2, 3, 5, 10, 50, 100, 200, 300} {
				for _, in := range []int64{0, 1, 100, c.samplesPerFrm - 1} {
					places = append(places, f*bpf+4*in)
				}
			}
			for i := int64(1); i <= 12; i++ {
				places = append(places, 4*65536*i) // where a player loading in pieces reads from
			}
			buf := make([]byte, 8192)
			bad := 0
			for _, at := range places {
				if at+int64(len(buf)) > int64(len(all)) {
					continue
				}
				if _, err := d.Seek(at, io.SeekStart); err != nil {
					t.Fatalf("seek to %d: %v", at, err)
				}
				if _, err := io.ReadFull(d, buf); err != nil {
					t.Fatalf("read at %d: %v", at, err)
				}
				if !bytes.Equal(buf, all[at:at+int64(len(buf))]) {
					bad++
					if bad <= 5 {
						t.Errorf("after a seek to byte %d (frame %d + %d samples) the samples differ", at, at/bpf, (at%bpf)/4)
					}
				}
			}
			if bad > 0 {
				t.Errorf("%d of %d seeks differ", bad, len(places))
			}
		})
	}
}

// TestSeekPastTheEnd: a seek to the end or past it is not an error; a read
// there gives io.EOF (it once indexed out of range).
func TestSeekPastTheEnd(t *testing.T) {
	d, _ := decodeAll(t, "example/mpeg2.mp3")
	for _, at := range []int64{d.Length(), d.Length() + 4, d.Length() + 4*1152*10} {
		if _, err := d.Seek(at, io.SeekStart); err != nil {
			t.Fatalf("seek to %d (length %d): %v", at, d.Length(), err)
		}
		if n, err := d.Read(make([]byte, 4096)); n != 0 || err != io.EOF {
			t.Errorf("read at %d (length %d): %d, %v", at, d.Length(), n, err)
		}
	}
	if _, err := d.Seek(-4, io.SeekStart); err == nil {
		t.Error("a seek before the start without an error")
	}
	// back from the end, it reads again
	if _, err := d.Seek(4*1152, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if n, err := d.Read(make([]byte, 4096)); n == 0 || err != nil {
		t.Errorf("read after seeking back: %d, %v", n, err)
	}
}
