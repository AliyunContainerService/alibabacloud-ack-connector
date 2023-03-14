package base

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

type Component struct {
	context.Context
	*logrus.Logger
	sync.Mutex
}

func NewComponent(ctx context.Context, logger *logrus.Logger) Component {
	return Component{Context: ctx, Logger: logger}
}
