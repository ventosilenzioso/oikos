package api

import (
	"context"

	obspb "github.com/oikos/oikos/gen/go/observability"
	"github.com/oikos/oikos/internal/observability"
	"github.com/oikos/oikos/internal/store"
)

type ObservabilityService struct {
	obspb.UnimplementedObservabilityServiceServer
	health  *observability.HealthChecker
	db      *store.DB
	metrics *observability.Metrics
}

func NewObservabilityService(health *observability.HealthChecker, metrics *observability.Metrics, db *store.DB) *ObservabilityService {
	return &ObservabilityService{health: health, metrics: metrics, db: db}
}

func (s *ObservabilityService) GetHealthStatus(ctx context.Context, _ *obspb.GetHealthStatusRequest) (*obspb.HealthStatus, error) {
	r := s.health.Check(ctx)
	out := &obspb.HealthStatus{Status: string(r.Status), TimestampUnix: r.Timestamp.Unix()}
	for _, c := range r.Checks {
		out.Checks = append(out.Checks, &obspb.HealthCheck{Name: c.Name, Status: c.Status, Error: c.Error})
	}
	return out, nil
}

func (s *ObservabilityService) GetServerMetrics(ctx context.Context, req *obspb.GetServerMetricsRequest) (*obspb.GetServerMetricsResponse, error) {
	servers, _, err := (observability.RuntimeSnapshotProvider{DB: s.db, Runtime: nil}).Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	out := &obspb.GetServerMetricsResponse{}
	for _, metric := range servers {
		if req.ServerId == "" || req.ServerId == metric.ID {
			out.Metrics = append(out.Metrics, &obspb.ServerMetric{ServerId: metric.ID, Status: metric.Status, CpuPercent: metric.CPU, MemoryBytes: metric.MemoryBytes})
		}
	}
	return out, nil
}

func (s *ObservabilityService) StreamEvents(req *obspb.StreamEventsRequest, stream obspb.ObservabilityService_StreamEventsServer) error {
	events, err := s.db.ListEvents(store.EventFilter{ServerID: req.ServerId, Type: req.Type, Limit: int(req.Limit)})
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := stream.Send(&obspb.EventRecord{Id: e.ID, Type: e.Type, ServerId: e.ServerID, Severity: e.Severity, Message: e.Message, Metadata: e.Metadata, CreatedAtUnix: e.CreatedAt.Unix()}); err != nil {
			return err
		}
	}
	return nil
}
