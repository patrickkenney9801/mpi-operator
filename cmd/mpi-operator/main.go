// Copyright 2018 The Kubeflow Authors.
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

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"k8s.io/klog"

	"github.com/kubeflow/mpi-operator/cmd/mpi-operator/app"
	"github.com/kubeflow/mpi-operator/cmd/mpi-operator/app/options"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	mpiServiceName = "mpi-operator"
	mpiTracer      = "github.com/patrickkenney9801/mpi-operator"
)

func startMonitoring(monitoringPort int) {
	if monitoringPort != 0 {
		go func() {
			klog.Infof("Setting up client for monitoring on port: %d", monitoringPort)
			http.Handle("/metrics", promhttp.Handler())
			err := http.ListenAndServe(fmt.Sprintf(":%d", monitoringPort), nil)
			if err != nil {
				klog.Error("Monitoring endpoint setup failure.", err)
			}
		}()
	}
}

func setupTracing(ctx context.Context, collectorEndpoint string, version string) (*tracesdk.TracerProvider, error) {
	spanExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(collectorEndpoint))
	if err != nil {
		return nil, err
	}
	traceResource :=
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(mpiServiceName),
			semconv.ServiceVersionKey.String(version),
		)
	traceProvider := tracesdk.NewTracerProvider(tracesdk.WithBatcher(spanExporter), tracesdk.WithResource(traceResource), tracesdk.WithSampler(tracesdk.AlwaysSample()))
	otel.SetTracerProvider(traceProvider)
	return traceProvider, nil
}

func main() {
	klog.InitFlags(nil)
	s := options.NewServerOption()
	s.AddFlags(flag.CommandLine)

	flag.Parse()

	startMonitoring(s.MonitoringPort)
	traceProvider, err := setupTracing(context.TODO(), "tempo.tempo:4317", "0.0.1")
	if err != nil {
		klog.Error("Tracing setup failure.", err)
	}

	if err := app.Run(s); err != nil {
		klog.Fatalf("%v\n", err)
	}
	_ = traceProvider.Shutdown(context.TODO())
}
