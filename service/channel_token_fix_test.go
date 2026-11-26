package service

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelTokenFixValidation(t *testing.T) {
	for _, tc := range []struct {
		body string
		bad  bool
	}{
		{`{}`, false}, {`{"tokenFix":0.65}`, false}, {`{"tokenFix":0.5}`, false},
		{`{"tokenFix":1.2}`, false}, {`{"tokenFix":0}`, true}, {`{"tokenFix":0.49}`, true}, {`{"tokenFix":-1}`, true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			ch := &model.Channel{Other: tc.body}
			err := ch.ValidateSettings()
			if tc.bad {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
