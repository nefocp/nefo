package database

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestNoAutoCreateUser(t *testing.T) {
	long := "INSERT INTO t VALUES ('" + strings.Repeat("NO_AUTO_CREATE_USER,", 10000) + "');\n"
	in := "/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;\n" +
		"/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION' */ ;\n" +
		"/*!50003 SET sql_mode              = 'NO_AUTO_CREATE_USER,STRICT_TRANS_TABLES' */ ;\n" +
		"/*!50003 SET sql_mode              = 'NO_AUTO_CREATE_USER' */ ;\n" +
		long +
		"/*!50003 SET sql_mode              = @saved_sql_mode */ ;"
	want := "/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;\n" +
		"/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION' */ ;\n" +
		"/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES' */ ;\n" +
		"/*!50003 SET sql_mode              = '' */ ;\n" +
		long +
		"/*!50003 SET sql_mode              = @saved_sql_mode */ ;"
	f := newNoAutoCreateUser(iotest.HalfReader(strings.NewReader(in)))
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Errorf("got %d bytes, want %d:\n%.400s", len(b), len(want), b)
	}
	if f.seen != 3 {
		t.Errorf("changed %d lines", f.seen)
	}
}
