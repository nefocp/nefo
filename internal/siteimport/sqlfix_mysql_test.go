package siteimport

import (
	"io"
	"strings"
	"testing"
)

func TestMySQLCollation(t *testing.T) {
	for in, want := range map[string]string{
		"utf8mb4_uca1400_ai_ci":               "utf8mb4_0900_ai_ci",
		"UTF8MB4_UCA1400_AI_CI":               "utf8mb4_0900_ai_ci",
		"utf8mb4_uca1400_as_cs":               "utf8mb4_0900_as_cs",
		"utf8mb4_uca1400_as_ci":               "utf8mb4_0900_as_ci",
		"utf8mb4_uca1400_ai_cs":               "utf8mb4_0900_as_cs",
		"utf8mb4_uca1400_nopad_ai_ci":         "utf8mb4_0900_ai_ci",
		"utf8mb4_uca1400_vietnamese_ai_ci":    "utf8mb4_vi_0900_ai_ci",
		"utf8mb4_uca1400_swedish_nopad_as_cs": "utf8mb4_sv_0900_as_cs",
		"utf8mb4_uca1400_german2_as_ci":       "utf8mb4_de_pb_0900_as_cs",
		"utf8mb4_uca1400_thai_ai_ci":          "utf8mb4_0900_ai_ci", // no MySQL language collation
		"utf8mb3_uca1400_ai_ci":               "utf8mb3_unicode_520_ci",
		"utf8_uca1400_as_cs":                  "utf8mb3_bin",
		"ucs2_uca1400_ai_ci":                  "ucs2_unicode_520_ci",
	} {
		if got, ok := mysqlCollation(in); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", in, got, ok, want)
		}
	}
	for _, keep := range []string{"utf8mb4_unicode_520_ci", "utf8mb4_0900_ai_ci", "uca1400_ai_ci", "latin1_uca1400_ai_ci", "utf8mb4_uca1400", "utf8mb4_uca1400_ai_xx", "x_uca1400_y"} {
		if got, ok := mysqlCollation(keep); ok {
			t.Errorf("%s changed to %s", keep, got)
		}
	}
}

func TestFixDumpForMySQL(t *testing.T) {
	in := "/*M!999999\\- enable the sandbox mode */\n" +
		"ALTER DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_uca1400_ai_ci;\n" +
		"CREATE TABLE `t` (`a` varchar(10) COLLATE utf8mb4_uca1400_ai_ci DEFAULT 'utf8mb4_uca1400_ai_ci') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;\n" +
		"/*!80016 DEFAULT ENCRYPTION='N' */;\n" +
		"SET @@GLOBAL.GTID_PURGED='x:1-2';\n"
	b, _ := io.ReadAll(FixDump(strings.NewReader(in), true))
	out := string(b)
	if strings.Count(out, "utf8mb4_0900_ai_ci") != 3 || !strings.Contains(out, "DEFAULT 'utf8mb4_uca1400_ai_ci'") {
		t.Errorf("collations (the string literal must stay):\n%s", out)
	}
	if !strings.Contains(out, "/*!80016 DEFAULT ENCRYPTION='N' */") {
		t.Errorf("MySQL's own comment changed for MySQL:\n%s", out)
	}
	if strings.Contains(out, "GTID_PURGED") {
		t.Errorf("GTID statement kept:\n%s", out)
	}
	// Into MariaDB the other way round, as before.
	b, _ = io.ReadAll(FixDump(strings.NewReader("CREATE TABLE t (a int) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci /*!80016 DEFAULT ENCRYPTION='N' */;\n"), false))
	if out := string(b); !strings.Contains(out, "COLLATE=utf8mb4_unicode_520_ci") || strings.Contains(out, "/*!80016") {
		t.Errorf("into MariaDB:\n%s", out)
	}
}

func TestFixCollationsKeepsStatements(t *testing.T) {
	in := "USE `other_db`;\nCREATE DATABASE x;\nSET @@GLOBAL.GTID_PURGED='x:1';\nCREATE TABLE t (a int) COLLATE=utf8mb4_0900_ai_ci /*!80016 DEFAULT ENCRYPTION='N' */;\n"
	b, _ := io.ReadAll(FixCollations(strings.NewReader(in), false))
	out := string(b)
	for _, want := range []string{"USE `other_db`;\n", "CREATE DATABASE x;\n", "GTID_PURGED", "COLLATE=utf8mb4_unicode_520_ci", "/* 80016 DEFAULT ENCRYPTION='N' */"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	b, _ = io.ReadAll(FixCollations(strings.NewReader("USE `a`;\nCREATE TABLE t (a int) COLLATE=utf8mb4_uca1400_ai_ci;\n"), true))
	if out := string(b); out != "USE `a`;\nCREATE TABLE t (a int) COLLATE=utf8mb4_0900_ai_ci;\n" {
		t.Errorf("into MySQL:\n%s", out)
	}
}
