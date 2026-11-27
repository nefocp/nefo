package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nefocp/nefo/internal/backup"
	"github.com/nefocp/nefo/internal/run"
)

// ---- decrypt-backup ----

// cmdDecryptBackup decrypts a copy that remote storage encrypted
// (FILE.tar.gz.enc), on any machine: only the passphrase is needed. The
// passphrase is read from a file or from standard input, never from the
// command line, where other users could see it.
func cmdDecryptBackup(args []string) error {
	fs := flag.NewFlagSet("decrypt-backup", flag.ExitOnError)
	passFile := fs.String("passphrase-file", "", "read the passphrase from this file (first line) instead of standard input")
	out := fs.String("o", "", `output file (default: FILE without ".enc"; "-" for standard output)`)
	force := fs.Bool("force", false, "replace the output file if it exists")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: nefo decrypt-backup [-passphrase-file FILE] [-o OUT] FILE.tar.gz.enc\n\n"+
			"Decrypts a backup copy that remote storage encrypted with a passphrase.\n"+
			"Without -passphrase-file, the passphrase is asked for (or read from the first line of standard input).\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	in := fs.Arg(0)
	if *out == "" {
		*out = strings.TrimSuffix(in, ".enc")
		if *out == in {
			*out = in + ".tar.gz"
		}
	}
	pass, err := readPassphrase(*passFile)
	if err != nil {
		return err
	}
	src, err := os.Open(in)
	if err != nil {
		return err
	}
	defer src.Close()

	var w io.Writer = os.Stdout
	var tmp *os.File
	if *out != "-" {
		if _, err := os.Lstat(*out); err == nil && !*force {
			return fmt.Errorf("%s exists: remove it or add -force", *out)
		}
		tmp, err = os.CreateTemp(filepath.Dir(*out), "."+filepath.Base(*out)+".*.part")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name()) // after the rename, nothing is left to remove
		defer tmp.Close()
		w = tmp
	}
	h := sha256.New()
	encrypted, err := backup.DecryptBackup(src, io.MultiWriter(w, h), pass)
	if err != nil {
		return err
	}
	if !encrypted {
		fmt.Fprintln(os.Stderr, "note: this file is not encrypted; it was copied as it is")
	}
	if tmp != nil {
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), *out); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Decrypted to %s\n", *out)
	}
	fmt.Fprintf(os.Stderr, "SHA-256: %s\n", hex.EncodeToString(h.Sum(nil)))
	return nil
}

// readPassphrase reads the first line of file, or of standard input, with
// a prompt and without echo when standard input is a terminal.
func readPassphrase(file string) (string, error) {
	var r io.Reader = os.Stdin
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	} else if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "Passphrase: ")
		ctx, sys := context.Background(), run.System{}
		if sys.Pipe(ctx, os.Stdin, io.Discard, "stty", "-echo") == nil {
			defer func() {
				sys.Pipe(ctx, os.Stdin, io.Discard, "stty", "echo")
				fmt.Fprintln(os.Stderr)
			}()
		}
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", errors.New("no passphrase given")
	}
	return line, nil
}
