package scheduler

import (
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Scheduler struct {
	cron   *cron.Cron
	logger *zap.Logger
}

func New(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		logger: logger,
	}
}

type JobFunc func(ctx context.Context) error

func (s *Scheduler) AddJob(schedule, name string, fn JobFunc) {
	s.cron.AddFunc(schedule, func() {
		s.logger.Info("running scheduled job", zap.String("job", name))
		ctx := context.Background()
		if err := fn(ctx); err != nil {
			s.logger.Error("scheduled job failed", zap.String("job", name), zap.Error(err))
		} else {
			s.logger.Info("scheduled job completed", zap.String("job", name))
		}
	})
}

func (s *Scheduler) Start() { s.cron.Start() }
func (s *Scheduler) Stop()  { s.cron.Stop() }
