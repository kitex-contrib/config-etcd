// Copyright 2023 CloudWeGo Authors
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

package degradation

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/thriftgo/pkg/test"
	"testing"
)

var errFake = errors.New("fake error")

func invoke(ctx context.Context, request, response interface{}) error {
	return errFake
}

func TestNewDegradationMiddleware(t *testing.T) {
	container := NewContainer()
	degradationMiddleware := NewDegradationMiddleware(container)
	test.Assert(t, errors.Is(degradationMiddleware(invoke)(context.Background(), nil, nil), errFake))
	container.NotifyPolicyChange(&Config{Enable: true, Percentage: 100})
	test.Assert(t, errors.Is(degradationMiddleware(invoke)(context.Background(), nil, nil), kerrors.ErrACL))
}
