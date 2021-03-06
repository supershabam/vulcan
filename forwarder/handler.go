// Copyright 2016 The Vulcan Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package forwarder

import (
	"io"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/storage/remote"
)

// WriterServer handles HTTP write requests from Prometheus.
type WriterServer interface {
	Write(context.Context, *remote.WriteRequest) (*remote.WriteResponse, error)
}

type decompressor func(io.Reader) io.Reader

// WriteHandler returns a Handler Interface for handling Prometheus remote
// WriteRequests.
func WriteHandler(f *Forwarder, compresstionType string) http.Handler {
	var decompr decompressor

	switch compresstionType {
	default:
		decompr = func(r io.Reader) io.Reader { return snappy.NewReader(r) }
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBuf, err := ioutil.ReadAll(decompr(r.Body))
		if err != nil {
			log.WithError(err).Error("unexpected error reading request")
			return
		}

		var req remote.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if _, err := f.Write(context.Background(), &req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
