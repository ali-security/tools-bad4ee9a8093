// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/internal/jsonrpc2"
	"golang.org/x/tools/internal/tool"
)

func TestListenParsing(t *testing.T) {
	tests := []struct {
		input, wantNetwork, wantAddr string
	}{
		{"127.0.0.1:0", "tcp", "127.0.0.1:0"},
		{"unix;/tmp/sock", "unix", "/tmp/sock"},
		{"auto", "auto", ""},
		{"auto;foo", "auto", "foo"},
	}

	for _, test := range tests {
		gotNetwork, gotAddr := parseAddr(test.input)
		if gotNetwork != test.wantNetwork {
			t.Errorf("network = %q, want %q", gotNetwork, test.wantNetwork)
		}
		if gotAddr != test.wantAddr {
			t.Errorf("addr = %q, want %q", gotAddr, test.wantAddr)
		}
	}
}

// runServe runs the serve command with the given command line flags and
// returns its result. It fails the test if the server does not return in a
// timely manner, so that a regression cannot hang the test binary.
func runServe(t *testing.T, args ...string) error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := New("gopls-test", "", nil, nil)
	app.Serve.app = app
	errc := make(chan error, 1)
	go func() {
		errc <- tool.Run(ctx, &app.Serve, args)
	}()
	select {
	case err := <-errc:
		return err
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatalf("serve %v did not return", args)
		return nil
	}
}

// TestListenImplicitAllInterfaces checks that a -listen address with an
// empty host, which would implicitly bind all network interfaces, is
// rejected rather than served.
func TestListenImplicitAllInterfaces(t *testing.T) {
	for _, listen := range []string{":0", "tcp;:0"} {
		err := runServe(t, "-listen="+listen, "-listen.timeout=100ms")
		if err == nil || !strings.Contains(err.Error(), "implicitly binds all network interfaces") {
			t.Errorf("serve -listen=%s: got error %v, want implicit all-interfaces error", listen, err)
		}
	}
}

// TestListenExplicitHost checks that a -listen address with an explicit host
// is still served.
func TestListenExplicitHost(t *testing.T) {
	for _, listen := range []string{"localhost:0", "tcp;127.0.0.1:0"} {
		err := runServe(t, "-listen="+listen, "-listen.timeout=100ms")
		if err != jsonrpc2.ErrIdleTimeout {
			t.Errorf("serve -listen=%s: got error %v, want %v", listen, err, jsonrpc2.ErrIdleTimeout)
		}
	}
}

// TestNoPortFlag checks that there is no -port flag, which used to listen on
// all network interfaces.
func TestNoPortFlag(t *testing.T) {
	seen := make(map[reflect.Type]bool)
	var check func(typ reflect.Type)
	check = func(typ reflect.Type) {
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Tag.Get("flag") == "port" {
				t.Errorf("%s.%s defines the -port flag", typ.Name(), field.Name)
			}
			check(field.Type)
		}
	}
	check(reflect.TypeOf(Application{}))
}
