package apiserver_test

import (
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
)

// TestAuthorizationMatrix calls every API method as every kind of caller,
// without any transport.
func TestAuthorizationMatrix(t *testing.T) {
	for _, tc := range apiservertest.Cases() {
		for _, caller := range apiservertest.Callers {
			t.Run(tc.Name+"/"+string(caller), func(t *testing.T) {
				t.Parallel()
				f := apiservertest.New(t)
				err := tc.Call(f.Context(t, caller), f.Server, f)
				if err := apiservertest.Check(err, tc.Expect(caller)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
