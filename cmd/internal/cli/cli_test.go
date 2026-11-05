// Copyright 2026 The go-lc3 Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func generatedWAV(t *testing.T, rate, ch, samples int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := writeWave(&b, rate, ch, uint64(samples)); err != nil {
		t.Fatal(err)
	}
	seed := uint32(0x13471)
	for i := 0; i < samples*ch; i++ {
		seed = seed*1664525 + 1013904223
		binary.Write(&b, binary.LittleEndian, int16(int(seed>>16)-32768))
	}
	return b.Bytes()
}

// Golden files were generated with the pinned liblc3 scalar reference tools,
// with reassociation and FP contraction disabled. Checks include all container
// bytes and output WAV bytes, rather than only round-trip audio length.
func TestReferenceTools(t *testing.T) {
	cases := []struct {
		rate, ch, samples int
		ms                float64
		bitrate           int
		hr                bool
		enc, dec          int
		encoded, decoded  string
	}{
		{16000, 1, 173, 10, 32000, false, 0, 0, "3d00e9d8e9702bae236c6fbf5eac8a6c6cc5e5bda616785898c2415be7da7698", "8d002f1269b4ea622b12ccada05fe31ce4ac5fe13e475fe0b34254216528522e"},
		{48000, 2, 733, 7.5, 128000, false, 0, 0, "bce52313c256c0e8e70373b97ee26b9323fa4c42e8191625e06fd0fc2e87926c", "fbdec1f202db4c5c876fb2f6045f58bbb29d7e732e20c8d0cfd573262a387ceb"},
		{48000, 1, 257, 2.5, 96000, false, 0, 0, "b31124133be3b0ea3fd67ee5e1bf5fb7856b5041c791ce11b1663023f6bd29c2", "cca63133f7495ce46e5fdb2c4da2a770a336fbdbe6db59d69e7b3a42cbbed8b6"},
		{96000, 2, 2001, 5, 400000, true, 0, 0, "29fe0c93a8548b9a4177692965147e1d379c394243e216099846619819a24c13", "43d93aacb3587500c81351cf246865f14a48e30a324f07dc0b2328548eabbf8b"},
		{48000, 2, 1763, 10, 64000, false, 16000, 48000, "651bb3afcc78c3f2cc623f3cca6aa7db0b26f0263f4d789efc6e4c99c3a4b124", "967f47322670b9298d7aee5e29e565943cc0de97ccecd0e8b5ac82ee633440e8"},
		{96000, 1, 1081, 2.5, 300000, true, 48000, 96000, "8b53d95dd7aa6488ee1727138ded556b4eb06b7ebdf16e550ac0ce76fa5ed6af", "240e5360fc6250df11f85951e13b2719f988385e8cd9f377eb3003bf0c236e3e"}}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d/%d/%g/%v/%d", tc.rate, tc.ch, tc.ms, tc.hr, tc.enc), func(t *testing.T) {
			source := generatedWAV(t, tc.rate, tc.ch, tc.samples)
			args := []string{"-b", fmt.Sprint(tc.bitrate), "-m", fmt.Sprint(tc.ms)}
			if tc.hr {
				args = append(args, "-H")
			}
			if tc.enc != 0 {
				args = append(args, "-r", fmt.Sprint(tc.enc))
			}
			var encoded, decoded, log bytes.Buffer
			if err := RunEncoder(args, bytes.NewReader(source), &encoded, &log); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())); got != tc.encoded {
				t.Errorf("LC3 SHA256 %s want %s", got, tc.encoded)
			}
			args = nil
			if tc.dec != 0 {
				args = []string{"-r", fmt.Sprint(tc.dec)}
			}
			if err := RunDecoder(args, bytes.NewReader(encoded.Bytes()), &decoded, &log); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(decoded.Bytes())); got != tc.decoded {
				t.Errorf("WAV SHA256 %s want %s", got, tc.decoded)
			}
		})
	}
}
func TestChannelsDelayAndTail(t *testing.T) {
	for _, ch := range []int{1, 2, 8} {
		for _, samples := range []int{0, 1, 39, 40, 119, 120, 121, 159, 160, 161, 321} {
			t.Run(fmt.Sprintf("%d/%d", ch, samples), func(t *testing.T) {
				var encoded, decoded, log bytes.Buffer
				if err := RunEncoder([]string{"-b", fmt.Sprint(ch * 32100)}, bytes.NewReader(generatedWAV(t, 16000, ch, samples)), &encoded, &log); err != nil {
					t.Fatal(err)
				}
				if err := RunDecoder(nil, bytes.NewReader(encoded.Bytes()), &decoded, &log); err != nil {
					t.Fatal(err)
				}
				if decoded.Len() != 44+samples*ch*2 {
					t.Fatalf("WAV length %d want %d", decoded.Len(), 44+samples*ch*2)
				}
				r, err := readWave(bytes.NewReader(decoded.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				if r.info.samples != uint32(samples) || r.info.channels != ch {
					t.Fatalf("metadata %+v", r.info)
				}
			})
		}
	}
}
func TestRejectTruncatedAndCorruptFiles(t *testing.T) {
	var encoded, log bytes.Buffer
	if err := RunEncoder([]string{"-b", "32000"}, bytes.NewReader(generatedWAV(t, 16000, 1, 321)), &encoded, &log); err != nil {
		t.Fatal(err)
	}
	good := encoded.Bytes()
	for _, n := range []int{0, 1, 17, 18, 19, len(good) - 1} {
		var out bytes.Buffer
		if err := RunDecoder(nil, bytes.NewReader(good[:n]), &out, &log); err == nil {
			t.Fatalf("accepted truncation to %d", n)
		}
	}
	bad := append([]byte(nil), good...)
	for i := 20; i < 60; i++ {
		bad[i] = 255
	}
	var out bytes.Buffer
	if err := RunDecoder(nil, bytes.NewReader(bad), &out, &log); err == nil {
		t.Fatal("accepted corrupt payload")
	}
}
func TestWaveChunksAndValidation(t *testing.T) {
	source := generatedWAV(t, 16000, 1, 173)
	junk := append([]byte(nil), source[:12]...)
	junk = append(junk, []byte{'J', 'U', 'N', 'K', 1, 0, 0, 0, 0xa5, 0}...)
	junk = append(junk, source[12:]...)
	binary.LittleEndian.PutUint32(junk[4:], uint32(len(junk)-8))
	var out, log bytes.Buffer
	if err := RunEncoder([]string{"-b", "32000"}, bytes.NewReader(junk), &out, &log); err != nil {
		t.Fatal(err)
	}
	changes := []func([]byte){func(b []byte) { b[0] = 'X' }, func(b []byte) { b[20] = 3 }, func(b []byte) { b[34] = 24 }, func(b []byte) { b[32] = 4 }, func(b []byte) { b[40] = 0xff; b[41] = 0xff }, func(b []byte) { b[40] = 1; b[41] = 0; b[42] = 0; b[43] = 0 }}
	for i, change := range changes {
		bad := append([]byte(nil), source...)
		change(bad)
		out.Reset()
		if err := RunEncoder([]string{"-b", "32000"}, bytes.NewReader(bad), &out, &log); err == nil {
			t.Fatalf("accepted invalid WAV %d", i)
		}
	}
	source = source[:len(source)-1]
	if err := RunEncoder([]string{"-b", "32000"}, bytes.NewReader(source), &out, &log); err == nil {
		t.Fatal("accepted truncated PCM")
	}
}
func TestArgumentsAndSameFile(t *testing.T) {
	for _, encode := range []bool{true, false} {
		var out, log bytes.Buffer
		run := RunDecoder
		if encode {
			run = RunEncoder
		}
		if err := run([]string{"-h"}, nil, &out, &log); err != nil {
			t.Fatal(err)
		}
		if log.Len() == 0 {
			t.Fatal("missing help")
		}
		for _, args := range [][]string{{"-missing"}, {"-r"}, {"a", "b", "c"}, {"-m", "NaN"}} {
			if err := run(args, nil, &out, &log); err == nil {
				t.Fatalf("accepted %v", args)
			}
		}
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "source.wav")
	source := generatedWAV(t, 16000, 1, 173)
	if err := os.WriteFile(name, source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := RunEncoder([]string{name, "-b", "32000", name}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted identical file")
	}
	got, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(got, source) {
		t.Fatal("input was modified")
	}
	target := filepath.Join(dir, "encoded.lc3")
	if err := RunEncoder([]string{name, "-b", "32000", target}, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestReference24BitTools(t *testing.T) {
	cases := []struct {
		rate, ch, samples int
		ms                float64
		bitrate           int
		hr                bool
		width             int
		encoded, decoded  string
	}{
		{32000, 2, 451, 7.5, 128000, false, 3, "dc62ef3c7d939597e94581abbc04590f751cafbfda119d19d82959452de901f5", "f10e79acabb0cf8e78737026ee940101ce583802ffe92b5c4883918cd485613e"},
		{48000, 1, 997, 10, 96000, false, 4, "662b657786eddc83de3d45318bf8f6da335fdda84ebb55ddbce9a13c26fe41d0", "637517a18da27fdd7c3020acb9f554bdc37dd47e9b7edc5efc8f6def77f15ad2"},
		{96000, 1, 577, 2.5, 300000, true, 3, "51786935910aed52ccbe76ae0973b9ddaa26dd826ba51087e33eb683b8142be8", "94d2f4ea3ca95f9b7b7b80ce1bb7ec5138acf835886e77c927897b9a529997ad"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d/%d/%g/%d", tc.rate, tc.ch, tc.ms, tc.width), func(t *testing.T) {
			var source, encoded, decoded, log bytes.Buffer
			if err := writeWaveFormat(&source, tc.rate, tc.ch, 24, tc.width, uint64(tc.samples)); err != nil {
				t.Fatal(err)
			}
			seed := uint32(0x13471)
			var word [4]byte
			for i := 0; i < tc.samples*tc.ch; i++ {
				seed = seed*1664525 + 1013904223
				v := int32(seed>>8) - (1 << 23)
				binary.LittleEndian.PutUint32(word[:], uint32(v))
				source.Write(word[:tc.width])
			}
			args := []string{"-b", fmt.Sprint(tc.bitrate), "-m", fmt.Sprint(tc.ms)}
			if tc.hr {
				args = append(args, "-H")
			}
			if err := RunEncoder(args, &source, &encoded, &log); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())); got != tc.encoded {
				t.Errorf("LC3 SHA256 %s want %s", got, tc.encoded)
			}
			if err := RunDecoder([]string{"-b", "24"}, &encoded, &decoded, &log); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(decoded.Bytes())); got != tc.decoded {
				t.Errorf("WAV SHA256 %s want %s", got, tc.decoded)
			}
		})
	}
}

func assertNoStagedOutput(t *testing.T, target string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary output files remain: %v, %v", matches, err)
	}
}
func syntheticLC3(t *testing.T) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := RunEncoder([]string{"-b", "32000"}, bytes.NewReader(generatedWAV(t, 16000, 1, 321)), &encoded, io.Discard); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}
func TestNamedOutputFailurePreservesExisting(t *testing.T) {
	wav := generatedWAV(t, 16000, 1, 321)
	encoded := syntheticLC3(t)
	corrupt := append([]byte(nil), encoded...)
	for i := 20; i < 60; i++ {
		corrupt[i] = 255
	}
	cases := []struct {
		name  string
		run   func([]string, io.Reader, io.Writer, io.Writer) error
		args  []string
		input []byte
	}{
		{"invalid duration", RunEncoder, []string{"-b", "32000", "-m", "3"}, wav},
		{"invalid WAV", RunEncoder, []string{"-b", "32000"}, []byte("invalid WAV")},
		{"truncated WAV", RunEncoder, []string{"-b", "32000"}, wav[:len(wav)-1]},
		{"invalid LC3", RunDecoder, nil, []byte("invalid LC3")},
		{"corrupt LC3", RunDecoder, nil, corrupt},
		{"truncated LC3", RunDecoder, nil, encoded[:len(encoded)-1]},
		{"invalid output rate", RunDecoder, []string{"-r", "8000"}, encoded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input")
			target := filepath.Join(dir, "output")
			original := []byte("preserve this existing output")
			if err := os.WriteFile(input, tc.input, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, original, 0640); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(target, 0640); err != nil {
				t.Fatal(err)
			}
			beforeInfo, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			expectedMode := beforeInfo.Mode().Perm()
			args := append(append([]string(nil), tc.args...), input, target)
			if err := tc.run(args, nil, io.Discard, io.Discard); err == nil {
				t.Fatal("expected failure")
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("existing output changed: %q, %v", got, err)
			}
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() != expectedMode {
				t.Fatalf("permissions changed: %v %v", info, err)
			}
			got, err = os.ReadFile(input)
			if err != nil || !bytes.Equal(got, tc.input) {
				t.Fatal("input changed")
			}
			assertNoStagedOutput(t, target)
			// A failed operation also leaves no destination when none existed.
			if err = os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err = tc.run(args, nil, io.Discard, io.Discard); err == nil {
				t.Fatal("expected failure")
			}
			if _, err = os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("failed operation created output: %v", err)
			}
			assertNoStagedOutput(t, target)
		})
	}
}
func TestNamedOutputCommitAndPermissions(t *testing.T) {
	wav := generatedWAV(t, 16000, 1, 321)
	expected := syntheticLC3(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "input.wav")
	target := filepath.Join(dir, "output.lc3")
	if err := os.WriteFile(input, wav, 0600); err != nil {
		t.Fatal(err)
	}
	for _, exists := range []bool{false, true} {
		expectedMode := os.FileMode(0600)
		if exists {
			if err := os.WriteFile(target, []byte("old output"), 0640); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(target, 0640); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			expectedMode = info.Mode().Perm()
		}
		if err := RunEncoder([]string{"-b", "32000", input, target}, nil, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(got, expected) {
			t.Fatalf("committed output differs: %v", err)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if (exists || runtime.GOOS != "windows") && info.Mode().Perm() != expectedMode {
			t.Fatalf("mode %o want %o", info.Mode().Perm(), expectedMode)
		}
		assertNoStagedOutput(t, target)
	}
}
func TestInputAliasesNeverReplaced(t *testing.T) {
	for _, decoder := range []bool{false, true} {
		for _, stdin := range []bool{false, true} {
			for _, alias := range []string{"same", "hardlink", "symlink"} {
				t.Run(fmt.Sprintf("decoder=%v/stdin=%v/%s", decoder, stdin, alias), func(t *testing.T) {
					original := generatedWAV(t, 16000, 1, 321)
					run := RunEncoder
					args := []string{"-b", "32000"}
					if decoder {
						original = syntheticLC3(t)
						run = RunDecoder
						args = nil
					}
					dir := t.TempDir()
					input := filepath.Join(dir, "input")
					target := input
					if err := os.WriteFile(input, original, 0600); err != nil {
						t.Fatal(err)
					}
					if alias != "same" {
						target = filepath.Join(dir, "alias")
						var err error
						if alias == "hardlink" {
							err = os.Link(input, target)
						} else {
							err = os.Symlink(input, target)
						}
						if err != nil {
							t.Skipf("links unavailable: %v", err)
						}
					}
					var reader io.Reader
					inputArg := input
					if stdin {
						f, err := os.Open(input)
						if err != nil {
							t.Fatal(err)
						}
						defer f.Close()
						reader = f
						inputArg = "-"
						defer func() {
							if _, err := f.Stat(); err != nil {
								t.Error("tool closed caller-owned stdin")
							}
						}()
					}
					args = append(args, inputArg, target)
					if err := run(args, reader, io.Discard, io.Discard); err == nil {
						t.Fatal("accepted input/output alias")
					}
					got, err := os.ReadFile(input)
					if err != nil || !bytes.Equal(got, original) {
						t.Fatalf("input changed: %v", err)
					}
					got, err = os.ReadFile(target)
					if err != nil || !bytes.Equal(got, original) {
						t.Fatalf("alias changed: %v", err)
					}
					if alias == "symlink" {
						info, err := os.Lstat(target)
						if err != nil || info.Mode()&os.ModeSymlink == 0 {
							t.Fatal("symlink replaced")
						}
					}
					assertNoStagedOutput(t, target)
				})
			}
		}
	}
}
func TestOutputSpecialFilesAndConcurrentChanges(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.wav")
	source := generatedWAV(t, 16000, 1, 321)
	if err := os.WriteFile(input, source, 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(dir, "directory")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := RunEncoder([]string{"-b", "32000", input, directory}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted directory output")
	}
	target := filepath.Join(dir, "output")
	for _, initiallyExists := range []bool{false, true} {
		os.Remove(target)
		if initiallyExists {
			if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		s, err := openStreams([]string{input, target}, nil, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.out.Write([]byte("staged output")); err != nil {
			t.Fatal(err)
		}
		// Simulate a separately replaced destination while conversion was running.
		if initiallyExists {
			if err = os.Rename(target, target+".old"); err != nil {
				t.Fatal(err)
			}
		}
		concurrent := []byte("newly created destination")
		if err = os.WriteFile(target, concurrent, 0600); err != nil {
			t.Fatal(err)
		}
		if err = s.commit(); err == nil {
			t.Fatal("overwrote a changed destination")
		}
		s.abort()
		got, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(got, concurrent) {
			t.Fatal("changed destination was lost")
		}
		assertNoStagedOutput(t, target)
	}
}
func TestNamedOutputFlushFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "output")
	original := []byte("original")
	if err := os.WriteFile(target, original, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := openStreams([]string{"-", target}, bytes.NewReader(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.out.Write([]byte("staged")); err != nil {
		t.Fatal(err)
	}
	// A real closed descriptor makes subsequent writes/Sync fail.
	if err = s.output.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.out.Write([]byte("more")); err == nil {
		t.Fatal("expected descriptor write error")
	}
	if err = s.commit(); err == nil {
		t.Fatal("committed after flush failure")
	}
	s.abort()
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("existing output lost after flush failure")
	}
	assertNoStagedOutput(t, target)
}

type failingPCMReader struct {
	reader  io.Reader
	allowed int
	failure error
}

func (r *failingPCMReader) Read(p []byte) (int, error) {
	if r.allowed == 0 {
		return 0, r.failure
	}
	if len(p) > r.allowed {
		p = p[:r.allowed]
	}
	n, err := r.reader.Read(p)
	r.allowed -= n
	return n, err
}
func TestNamedOutputReadFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "output.lc3")
	original := []byte("original file")
	if err := os.WriteFile(target, original, 0600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("injected PCM read failure")
	reader := &failingPCMReader{reader: bytes.NewReader(generatedWAV(t, 16000, 1, 321)), allowed: 44 + 60, failure: sentinel}
	err := RunEncoder([]string{"-b", "32000", "-", target}, reader, io.Discard, io.Discard)
	if !errors.Is(err, sentinel) {
		t.Fatalf("lost read error: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("existing output changed after read failure")
	}
	assertNoStagedOutput(t, target)
}
