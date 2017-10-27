package endpoint

import (
	"context"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	rl "github.com/juju/ratelimit"
	m_order "github.com/laidingqing/dabanshan/svcs/order/model"
	"github.com/laidingqing/dabanshan/svcs/order/service"
	stdopentracing "github.com/opentracing/opentracing-go"
	"github.com/sony/gobreaker"
)

// Set collects all of the endpoints that compose an add service. It's meant to
// be used as a helper struct, to collect all of the endpoints into a single
// parameter.
type Set struct {
	CreateOrderEndpoint endpoint.Endpoint
}

// New returns a Set that wraps the provided server, and wires in all of the
// expected endpoint middlewares via the various parameters.
func New(svc service.Service, logger log.Logger, duration metrics.Histogram, trace stdopentracing.Tracer) Set {
	var (
		createOrderEndpoint endpoint.Endpoint
	)
	{
		createOrderEndpoint = MakeCreateOrderEndpoint(svc)
		createOrderEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(1, 1))(createOrderEndpoint)
		createOrderEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(createOrderEndpoint)
		createOrderEndpoint = opentracing.TraceServer(trace, "CreateOrder")(createOrderEndpoint)
		createOrderEndpoint = LoggingMiddleware(log.With(logger, "method", "CreateOrder"))(createOrderEndpoint)
		createOrderEndpoint = InstrumentingMiddleware(duration.With("method", "CreateOrder"))(createOrderEndpoint)
	}

	return Set{
		CreateOrderEndpoint: createOrderEndpoint,
	}
}

// CreateOrder implements the service interface, so Set may be used as a service.
func (s Set) CreateOrder(ctx context.Context, a m_order.CreateOrderRequest) (m_order.CreatedOrderResponse, error) {
	resp, err := s.CreateOrderEndpoint(ctx, a)
	if err != nil {
		return m_order.CreatedOrderResponse{}, err
	}
	response := resp.(m_order.CreatedOrderResponse)
	return response, response.Err
}

// MakeCreateOrderEndpoint constructs a CreateOrder endpoint wrapping the service.
func MakeCreateOrderEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(m_order.CreateOrderRequest)
		v, err := s.CreateOrder(ctx, req)
		return v, err
	}
}
