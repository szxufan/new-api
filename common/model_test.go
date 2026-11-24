package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestSambertTTSEndpointCapability(t *testing.T) {
	assert.True(t, IsTTSSpeechModel("sambert-zhichu-v1"))
	assert.True(t, IsTTSSpeechModel("SAMBERT-zhimiao-emo-v1"))

	endpointTypes := GetEndpointTypesByChannelType(constant.ChannelTypeAli, "sambert-zhichu-v1")
	assert.Contains(t, endpointTypes, constant.EndpointTypeTTS)
}
