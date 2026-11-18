package preseed

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

// The mappings use both line endings and carry the comments Mojang puts in
const sampleMappings = "# (c) 2020 Microsoft Corporation.\r\n" +
	"net.example.Thing -> a:\r\n" +
	"# {\"fileName\":\"Thing.java\",\"id\":\"sourceFile\"}\n" +
	"    java.lang.String name -> b\n" +
	"    int[] counts -> a\n" +
	"    net.example.Thing$Inner inner -> c # trailing comment\n" +
	"    1:2:java.lang.Object get(int) -> a\n" +
	"    3:4:java.lang.Object get(long) -> a\n" +
	"    5:5:java.lang.Object get() -> b\n" +
	"    6:6:java.lang.Object get(java.lang.String):10:11 -> c\n" +
	"    7:7:void <init>() -> <init>\n" +
	"    8:8:java.lang.Object get(int) -> a\n" +
	"\n" +
	"net.example.Thing$Inner -> a$a:\n" +
	"    boolean flag -> a\n" +
	"net.example.Another -> b:"

// Overloads follow the buckets of a 16-slot java.util.HashMap: get() is in
// bucket 1, get(String) in 4, get(long) in 6, get(int) in 7
const sampleLegacyTSRG = "net/example/Another b\n" +
	"net/example/Thing a\n" +
	"\tcounts a\n" +
	"\tinner c\n" +
	"\tname b\n" +
	"\t<init> ()V <init>\n" +
	"\tget ()Ljava/lang/Object; b\n" +
	"\tget (Ljava/lang/String;)Ljava/lang/Object; c\n" +
	"\tget (J)Ljava/lang/Object; a\n" +
	"\tget (I)Ljava/lang/Object; a\n" +
	"net/example/Thing$Inner a$a\n" +
	"\tflag a\n"

const sampleTSRG = "net/example/Another b\n" +
	"net/example/Thing a\n" +
	"\tcounts a\n" +
	"\tname b\n" +
	"\tinner c\n" +
	"\t<init> ()V <init>\n" +
	"\tget ()Ljava/lang/Object; b\n" +
	"\tget (I)Ljava/lang/Object; a\n" +
	"\tget (J)Ljava/lang/Object; a\n" +
	"\tget (Ljava/lang/String;)Ljava/lang/Object; c\n" +
	"net/example/Thing$Inner a$a\n" +
	"\tflag a\n"

func TestSanitizeMappings(t *testing.T) {
	for name, want := range map[string]string{"legacy": sampleLegacyTSRG, "current": sampleTSRG} {
		got, ok, err := SanitizeMappings(strings.NewReader(sampleMappings), sha1Hex(want))
		if err != nil {
			t.Fatal(err)
		}
		if !ok || string(got) != want {
			t.Errorf("%s ordering: matched %v, got\n%s", name, ok, got)
		}
	}

	if _, ok, _ := SanitizeMappings(strings.NewReader(sampleMappings), sha1Hex("other")); ok {
		t.Error("a checksum no ordering meets was matched")
	}
}

func TestSanitizeMappingsRejectsGarbage(t *testing.T) {
	for _, input := range []string{
		"    int orphan -> a\n",
		"a.B -> c:\n    int x\n",
		"a.B -> c:\n    x:1:void f() -> a\n",
	} {
		if _, _, err := SanitizeMappings(strings.NewReader(input), "x"); err == nil {
			t.Errorf("%q was accepted", input)
		}
	}
}

func TestJavaHash(t *testing.T) {
	if got := javaHashSpread("hello"); got != 99162322^(99162322>>16) {
		t.Errorf("hello spreads to %d", got)
	}
	if javaHashSpread("Aa") != javaHashSpread("BB") {
		t.Error("String.hashCode collisions must collide")
	}
	for n, want := range map[int]int32{0: 16, 12: 16, 13: 32, 24: 32, 25: 64} {
		if got := javaHashMapCapacity(n); got != want {
			t.Errorf("capacity for %d entries = %d, want %d", n, got, want)
		}
	}
}

func TestStripMappingComment(t *testing.T) {
	for in, want := range map[string]string{
		"# header":            "",
		"a -> b: # x":         "a -> b:",
		"    int x -> a  # y": "    int x -> a",
		"    # indented":      " ",
		"plain":               "plain",
	} {
		if got := stripMappingComment(in); got != want {
			t.Errorf("stripMappingComment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanJavaLines(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("a\r\nb\rc\n\nd\r"))
	scanner.Split(scanJavaLines)

	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	if want := []string{"a", "b", "c", "", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}
