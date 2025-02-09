package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/slsa-framework/slsa-github-generator-go/pkg"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func errCmp(e1, e2 error) bool {
	return errors.Is(e1, e2) || errors.Is(e2, e1)
}

func Test_runVerify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		subject  string
		name     string
		config   string
		evalEnvs string
		err      error
		commands []string
		envs     []string
	}{
		{
			name:     "two ldflags",
			subject:  "binary-linux-amd64",
			config:   "./testdata/two-ldflags.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-trimpath",
				"-tags=netgo",
				"-ldflags=bla something-else",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
				"GO111MODULE=on",
				"CGO_ENABLED=0",
			},
		},
		{
			name:     "two ldflags empty env",
			subject:  "binary-linux-amd64",
			config:   "./testdata/two-ldflags-emptyenv.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-trimpath",
				"-tags=netgo",
				"-ldflags=bla something-else",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
			},
		},
		{
			name:     "two ldflags no env",
			subject:  "binary-linux-amd64",
			config:   "./testdata/two-ldflags-noenv.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-trimpath",
				"-tags=netgo",
				"-ldflags=bla something-else",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
			},
		},
		{
			name:     "two ldflags empty flags",
			subject:  "binary-linux-amd64",
			config:   "./testdata/two-ldflags-emptyflags.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-ldflags=bla something-else",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
				"GO111MODULE=on",
				"CGO_ENABLED=0",
			},
		},
		{
			name:     "two ldflags no flags",
			subject:  "binary-linux-amd64",
			config:   "./testdata/two-ldflags-noflags.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-ldflags=bla something-else",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
				"GO111MODULE=on",
				"CGO_ENABLED=0",
			},
		},
		{
			name:     "one ldflags",
			subject:  "binary-linux-amd64",
			config:   "./testdata/one-ldflags.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-trimpath",
				"-tags=netgo",
				"-ldflags=something-else",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
				"GO111MODULE=on",
				"CGO_ENABLED=0",
			},
		},
		{
			name:     "no ldflags",
			subject:  "binary-linux-amd64",
			config:   "./testdata/two-ldflags-noldflags.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-trimpath",
				"-tags=netgo",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
				"GO111MODULE=on",
				"CGO_ENABLED=0",
			},
		},
		{
			name:     "empty ldflags",
			subject:  "binary-linux-amd64",
			config:   "./testdata/emptyldflags.yml",
			evalEnvs: "VERSION_LDFLAGS:bla, ELSE:else",
			commands: []string{
				"-trimpath",
				"-tags=netgo",
				"-o",
				"binary-linux-amd64",
			},
			envs: []string{
				"GOOS=linux",
				"GOARCH=amd64",
				"GO111MODULE=on",
				"CGO_ENABLED=0",
			},
		},
	}

	for _, tt := range tests {
		tt := tt // Re-initializing variable so it is not changed while executing the closure below
		t.Run(tt.name, func(t *testing.T) {
			// *** WARNING: do not enable t.Parallel(), because we're redirecting stdout ***.

			// http://craigwickesser.com/2015/01/capture-stdout-in-go/
			r := runNew()
			r.start()

			err := runBuild(true,
				tt.config,
				tt.evalEnvs)

			s := r.end()

			if !errCmp(err, tt.err) {
				t.Errorf(cmp.Diff(err, tt.err, cmpopts.EquateErrors()))
			}

			cmd, env, subject, err := extract(s)
			if err != nil {
				t.Errorf("extract: %v", err)
			}

			goc, err := exec.LookPath("go")
			if err != nil {
				t.Errorf("exec.LookPath: %v", err)
			}

			if !cmp.Equal(subject, tt.subject) {
				t.Errorf(cmp.Diff(subject, tt.subject))
			}

			commands := append([]string{goc, "build", "-mod=vendor"}, tt.commands...)
			if !cmp.Equal(cmd, commands) {
				t.Errorf(cmp.Diff(cmd, commands))
			}

			sorted := cmpopts.SortSlices(func(a, b string) bool { return a < b })
			if !cmp.Equal(env, tt.envs, sorted) {
				t.Errorf(cmp.Diff(env, tt.envs))
			}
		})
	}
}

type run struct {
	oldStdout *os.File
	wPipe     *os.File
	rPipe     *os.File
}

func runNew() *run {
	return &run{}
}

func (r *run) start() {
	old := os.Stdout
	rp, wp, _ := os.Pipe()
	os.Stdout = wp

	r.oldStdout = old
	r.wPipe = wp
	r.rPipe = rp
}

func (r *run) end() string {
	r.wPipe.Close()
	os.Stdout = r.oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r.rPipe)
	s := buf.String()

	r.oldStdout = nil
	r.wPipe = nil
	r.rPipe = nil
	return s
}

func extract(lines string) ([]string, []string, string, error) {
	rsubject := regexp.MustCompile("^::set-output name=go-binary-name::(.*)$")
	rcmd := regexp.MustCompile("^::set-output name=go-command::(.*)$")
	renv := regexp.MustCompile("^::set-output name=go-env::(.*)$")
	var subject string
	var scmd string
	var senv string

	scanner := bufio.NewScanner(bytes.NewReader([]byte(lines)))
	for scanner.Scan() {
		n := rsubject.FindStringSubmatch(scanner.Text())
		if len(n) > 1 {
			subject = n[1]
		}

		c := rcmd.FindStringSubmatch(scanner.Text())
		if len(c) > 1 {
			scmd = c[1]
		}

		e := renv.FindStringSubmatch(scanner.Text())
		if len(e) > 1 {
			senv = e[1]
		}

		if subject != "" && scmd != "" && senv != "" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return []string{}, []string{}, "", err
	}

	cmd, err := pkg.UnmarshallList(scmd)
	if err != nil {
		return []string{}, []string{}, "", err
	}

	env, err := pkg.UnmarshallList(senv)
	if err != nil {
		return []string{}, []string{}, "", err
	}

	return cmd, env, subject, nil
}
