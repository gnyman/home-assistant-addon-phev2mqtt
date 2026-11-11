package client

import (
	"bytes"
	"strings"
	"testing"

	"github.com/buxtronix/phev2mqtt/protocol"
	log "github.com/sirupsen/logrus"
)

func TestProtocolLogsFilterOnlyPings(t *testing.T) {
	logger := log.StandardLogger()
	oldOut, oldLevel := logger.Out, logger.GetLevel()
	var output bytes.Buffer
	logger.SetOutput(&output)
	logger.SetLevel(log.TraceLevel)
	defer func() { logger.SetOutput(oldOut); logger.SetLevel(oldLevel) }()
	for _, enabled := range []bool{false, true} {
		c, _ := New(LogPingsOption(enabled))
		for _, direction := range []string{"SEND", "RECV"} {
			for _, typ := range []byte{protocol.CmdOutPingReq, protocol.CmdInPingResp} {
				output.Reset()
				c.logMessage(direction, &protocol.PhevMessage{Type: typ}, []byte{1, 2})
				if (output.Len() > 0) != enabled {
					t.Fatal("ping filter did not apply to both decoded and raw logs")
				}
			}
			for _, msg := range []*protocol.PhevMessage{
				{Type: protocol.CmdOutSend, Register: 0x1b, Data: []byte{2, 3, 2, 0}},
				{Type: protocol.CmdInResp, Ack: protocol.Ack, Register: 0x1b},
				{Type: protocol.CmdInBadEncoding, Data: []byte{0x45}},
				{Type: protocol.CmdInBadEncoding},
			} {
				output.Reset()
				c.logMessage(direction, msg, []byte{1, 2})
				if !strings.Contains(output.String(), "PHEV_TCP_"+direction+"_MSG") {
					t.Fatal("non-ping diagnostic was suppressed")
				}
			}
		}
		c.Close()
	}
}
