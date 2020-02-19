// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/pushgateway/storage"
)

// Delete returns a handler that accepts delete requests.
//
// The returned handler is already instrumented for Prometheus.
func Delete(ms storage.MetricStore, jobBase64Encoded bool, logger log.Logger) func(http.ResponseWriter, *http.Request) {
	var ctx context.Context
	var mtx sync.Mutex // Protects ps.

	instrumentedHandler := promhttp.InstrumentHandlerCounter(
		httpCnt.MustCurryWith(prometheus.Labels{"handler": "delete"}),
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			job := urlParams(ctx, "job")
			if jobBase64Encoded {
				var err error
				if job, err = decodeBase64(job); err != nil {
					http.Error(w, fmt.Sprintf("invalid base64 encoding in job name %q: %v", job, err), http.StatusBadRequest)
					level.Debug(logger).Log("msg", "invalid base64 encoding in job name", "job", job, "err", err.Error())
					return
				}
			}
			labelsString := urlParams(ctx, "labels")
			mtx.Unlock()

			labels, err := splitLabels(labelsString)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				level.Debug(logger).Log("msg", "failed to parse URL", "url", labelsString, "err", err.Error())
				return
			}
			if job == "" {
				http.Error(w, "job name is required", http.StatusBadRequest)
				level.Debug(logger).Log("msg", "job name is required")
				return
			}
			labels["job"] = job
			ms.SubmitWriteRequest(storage.WriteRequest{
				Labels:    labels,
				Timestamp: time.Now(),
			})
			w.WriteHeader(http.StatusAccepted)
		}),
	)

	return func(w http.ResponseWriter, r *http.Request) {
		mtx.Lock()
		ctx = r.Context()
		instrumentedHandler.ServeHTTP(w, r)
	}
}
