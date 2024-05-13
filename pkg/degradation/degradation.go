package degradation

import (
	"context"
	"errors"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

func NewDegradationMiddleware(container *Container) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) error {
			if e := container.GetAclRule()(ctx, request); e != nil {
				if !errors.Is(e, kerrors.ErrACL) {
					e = kerrors.ErrACL.WithCause(e)
				}
				return e
			}
			return next(ctx, request, response)
		}
	}
}
