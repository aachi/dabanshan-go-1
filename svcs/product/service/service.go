package service

import (
	"context"
	"errors"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/laidingqing/dabanshan/pb"
)

// Storage
var (
	mem map[int64]map[int64]*pb.ProductRecord
	mu  sync.RWMutex
)

func init() {
	mem = make(map[int64]map[int64]*pb.ProductRecord)
}

// Service describes a service that adds things together.
type Service interface {
	GetProducts(ctx context.Context, a, b int64) (int64, error)
}

// New returns a basic Service with all of the expected middlewares wired in.
func New(logger log.Logger, ints, chars metrics.Counter) Service {
	var svc Service
	{
		svc = NewBasicService()
		svc = LoggingMiddleware(logger)(svc)
		svc = InstrumentingMiddleware(ints, chars)(svc)
	}
	return svc
}

var (
	// ErrTwoZeroes ..
	ErrTwoZeroes = errors.New("can't sum two zeroes")
	// ErrIntOverflow ...
	ErrIntOverflow = errors.New("integer overflow")
	// ErrMaxSizeExceeded ...
	ErrMaxSizeExceeded = errors.New("result exceeds maximum size")
)

const (
	intMax = 1<<31 - 1
	intMin = -(intMax + 1)
	maxLen = 10
)

// NewBasicService returns a naïve, stateless implementation of Service.
func NewBasicService() Service {
	return basicService{}
}

type basicService struct{}

func (s basicService) GetProducts(_ context.Context, a, b int64) (int64, error) {
	if a == 0 && b == 0 {
		return 0, ErrTwoZeroes
	}
	if (b > 0 && a > (intMax-b)) || (b < 0 && a < (intMin-b)) {
		return 0, ErrIntOverflow
	}
	return a + b, nil
}