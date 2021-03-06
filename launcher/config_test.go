package launcher

import (
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/daemon/maintenance"
	"github.com/HouzuoGuo/laitos/daemon/plainsocket"
	"github.com/HouzuoGuo/laitos/daemon/smtpd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/sockd"
	"github.com/HouzuoGuo/laitos/daemon/telegrambot"
	"testing"
	"time"
)

var sampleConfigJSON = `
{
  "DNSDaemon": {
    "Address": "127.0.0.1",
    "AllowQueryIPPrefixes": [
      "192"
    ],
    "PerIPLimit": 5,
    "TCPPort": 45115,
    "UDPPort": 23518
  },
  "Features": {
    "Shell": {
      "InterpreterPath": "/bin/bash"
    }
  },
  "HTTPDaemon": {
    "Address": "127.0.0.1",
    "PerIPLimit": 10,
    "Port": 23486,
    "ServeDirectories": {
      "/dir": "/tmp/test-laitos-dir",
      "/my/dir": "/tmp/test-laitos-dir"
    }
  },
  "HTTPFilters": {
    "LintText": {
      "CompressSpaces": true,
      "CompressToSingleLine": true,
      "KeepVisible7BitCharOnly": true,
      "MaxLength": 35,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "httpshortcut": ".secho httpshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "alpha",
          "beta"
        ]
      ]
    }
  },
  "HTTPHandlers": {
    "CommandFormEndpoint": "/cmd_form",
    "GitlabBrowserEndpoint": "/gitlab",
    "GitlabBrowserEndpointConfig": {
      "PrivateToken": "just a dummy token"
    },
    "IndexEndpointConfig": {
      "HTMLFilePath": "/tmp/test-laitos-index.html"
    },
    "IndexEndpoints": [
      "/html",
      "/"
    ],
    "InformationEndpoint": "/info",
    "MailMeEndpoint": "/mail_me",
    "MailMeEndpointConfig": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "MicrosoftBotEndpoint1": "/microsoft_bot",
    "MicrosoftBotEndpointConfig1": {
      "ClientAppID": "dummy id",
      "ClientAppSecret": "dummy secret"
    },
    "TwilioCallEndpoint": "/call_greeting",
    "TwilioCallEndpointConfig": {
      "CallGreeting": "Hi there"
    },
    "TwilioSMSEndpoint": "/sms",
    "WebProxyEndpoint": "/proxy"
  },
  "MailClient": {
    "MTAHost": "127.0.0.1",
    "MTAPort": 25,
    "MailFrom": "howard@localhost"
  },
  "MailCommandRunner": {
    "CommandTimeoutSec": 10
  },
  "MailDaemon": {
    "Address": "127.0.0.1",
    "ForwardTo": [
      "howard@localhost",
      "root@localhost"
    ],
    "MyDomains": [
      "example.com",
      "howard.name"
    ],
    "PerIPLimit": 5,
    "Port": 18573
  },
  "MailFilters": {
    "LintText": {
      "CompressToSingleLine": true,
      "MaxLength": 70,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "mailshortcut": ".secho mailshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "aaa",
          "bbb"
        ]
      ]
    }
  },
  "Maintenance": {
    "IntervalSec": 3600,
    "Recipients": [
      "howard@localhost"
    ],
    "TCPPorts": [
      9114
    ]
  },
  "PlainSocketDaemon": {
    "Address": "127.0.0.1",
    "PerIPLimit": 5,
    "TCPPort": 17011,
    "UDPPort": 43915
  },
  "PlainSocketFilters": {
    "LintText": {
      "CompressToSingleLine": false,
      "MaxLength": 120,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "plainsocketshortcut": ".secho plainsockethortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "iii",
          "jjj"
        ]
      ]
    }
  },
  "SockDaemon": {
    "Address": "127.0.0.1",
    "Password": "1234567",
    "PerIPLimit": 10,
    "TCPPort": 6891,
    "UDPPort": 9122
  },
  "SupervisorNotificationRecipients": [
    "howard@localhost"
  ],
  "TelegramBot": {
    "AuthorizationToken": "intentionally-bad-token",
    "PerUserLimit": 2
  },
  "TelegramFilters": {
    "LintText": {
      "CompressToSingleLine": true,
      "MaxLength": 120,
      "TrimSpaces": true
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "PINAndShortcuts": {
      "PIN": "verysecret",
      "Shortcuts": {
        "telegramshortcut": ".secho telegramshortcut"
      }
    },
    "TranslateSequences": {
      "Sequences": [
        [
          "123",
          "456"
        ]
      ]
    }
  }
}`

// Most of the daemon test cases are copied from their own unit tests.
func TestConfig(t *testing.T) {
	var config Config
	if err := config.DeserialiseFromJSON([]byte(sampleConfigJSON)); err != nil {
		t.Fatal(err)
	}

	dnsDaemon := config.GetDNSD()
	dnsd.TestUDPQueries(dnsDaemon, t)
	dnsd.TestTCPQueries(dnsDaemon, t)

	maintenance.TestMaintenance(config.GetMaintenance(), t)

	httpDaemon := config.GetHTTPD()
	// HTTP daemon is expected to start in two seconds
	go func() {
		if err := httpDaemon.StartAndBlockNoTLS(0); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)
	httpd.TestHTTPD(httpDaemon, t)
	httpd.TestAPIHandlers(httpDaemon, t)

	mailcmd.TestCommandRunner(config.GetMailCommandRunner(), t)

	smtpd.TestSMTPD(config.GetMailDaemon(), t)

	plainsocket.TestTCPServer(config.GetPlainSocketDaemon(), t)
	plainsocket.TestUDPServer(config.GetPlainSocketDaemon(), t)

	sockd.TestSockd(config.GetSockDaemon(), t)

	telegrambot.TestTelegramBot(config.GetTelegramBot(), t)
}
