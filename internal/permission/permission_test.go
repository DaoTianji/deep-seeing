package permission_test

import (
	"testing"

	"deep-seeing/internal/permission"
)

func TestClassifyTiers(t *testing.T) {
	if permission.Classify("search_episodes") != permission.Observe {
		t.Fatal("search should be observe")
	}
	if permission.Classify("write_episode") != permission.Internal {
		t.Fatal("write should be internal")
	}
	if permission.Classify("payment") != permission.External {
		t.Fatal("payment external")
	}
	if permission.Classify("search_web") != permission.External {
		t.Fatal("search_web external")
	}
	if permission.Classify("read_webpage") != permission.External {
		t.Fatal("read_webpage external")
	}
	if permission.Classify("list_sources") != permission.Observe {
		t.Fatal("list_sources observe")
	}
	if permission.AllowAuto("payment") {
		t.Fatal("payment must interrupt")
	}
	if !permission.AllowAuto("get_time") {
		t.Fatal("get_time auto ok")
	}
}
