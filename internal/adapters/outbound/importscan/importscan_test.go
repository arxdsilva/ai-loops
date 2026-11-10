package importscan_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/importscan"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// writeTree writes files (slash paths) under a temp root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scan(t *testing.T, files map[string]string, p pack.Pack) archrule.Module {
	t.Helper()
	m, err := importscan.New().Scan(context.Background(), writeTree(t, files), p)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// evaluate runs the rules the way archcheck does for a regex pack without
// configured layers.
func evaluate(m archrule.Module, p pack.Pack) archrule.Report {
	return archrule.EvaluateWith(m, archrule.ConventionalLayers(p.Arch.Resolver), archrule.Rules{
		ForbiddenInDomain: p.Arch.ForbiddenInDomain, ImpureCalls: p.Arch.ImpureCalls,
		ThirdPartyForbiddenIn: p.Arch.ThirdPartyForbiddenIn,
	})
}

// summary renders error findings as "file:line rule" sorted.
func summary(r archrule.Report) []string {
	var out []string
	for _, f := range r.Findings {
		if f.Severity == archrule.SeverityError {
			out = append(out, fmt.Sprintf("%s:%d %s", f.File, f.Line, f.Rule))
		}
	}
	sort.Strings(out)
	return out
}

func wantFindings(t *testing.T, r archrule.Report, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := summary(r); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant:\n%s\nall: %+v", strings.Join(got, "\n"), strings.Join(want, "\n"), r.Findings)
	}
}

// imports maps "file:line" to "kind dir" for every import of the module.
func imports(m archrule.Module) map[string]string {
	out := map[string]string{}
	for _, p := range m.Packages {
		for _, i := range p.Imports {
			out[fmt.Sprintf("%s:%d %s", i.Pos.File, i.Pos.Line, i.Path)] = strings.TrimSpace(i.Kind + " " + i.Dir)
		}
	}
	return out
}

func wantImport(t *testing.T, m archrule.Module, key, want string) {
	t.Helper()
	got, ok := imports(m)[key]
	if !ok {
		t.Fatalf("no import %s; have %v", key, imports(m))
	}
	if got != want {
		t.Errorf("import %s = %q, want %q", key, got, want)
	}
}

func regexPack(id, resolver, importRe string, src, test []string, forbidden, impure []string) pack.Pack {
	return pack.Pack{ID: id, SourceGlobs: src, TestGlobs: test, Arch: pack.Arch{
		Parser: pack.ParserRegex, ImportRegex: importRe, Resolver: resolver,
		ForbiddenInDomain: forbidden, ImpureCalls: impure,
	}}
}

var javaPack = regexPack("java", "package-dir",
	`^\s*import\s+(?:static\s+)?([A-Za-z_][\w.]*)(?:\.\*)?\s*;`,
	[]string{"**/src/main/java/**/*.java"}, []string{"**/src/test/java/**/*.java", "**/*Test.java"},
	[]string{"org.springframework", "java.io"}, []string{"Instant.now", "System.currentTimeMillis"})

func TestJavaPackageDir(t *testing.T) {
	m := scan(t, map[string]string{
		"src/main/java/com/acme/domain/Order.java": `package com.acme.domain;

import com.acme.adapters.db.OrderRow;
import java.util.List;
import org.springframework.stereotype.Service;
import static com.acme.domain.Money.of;
// import com.acme.adapters.db.Ignored;

/**
 * Instant.now in a doc comment is not a call.
 */
public class Order { long at = Instant.now().toEpochMilli(); }
`,
		"src/main/java/com/acme/domain/Money.java":         "package com.acme.domain;\npublic class Money {}\n",
		"src/main/java/com/acme/adapters/db/OrderRow.java": "package com.acme.adapters.db;\nimport com.acme.domain.Order;\nclass OrderRow { long t = Instant.now(); }\n",
		"src/test/java/com/acme/domain/OrderTest.java":     "package com.acme.domain;\nimport com.acme.adapters.db.OrderRow;\n",
		"target/classes/Gen.java":                          "package x;\nimport com.acme.adapters.db.OrderRow;\n",
	}, javaPack)
	wantImport(t, m, "src/main/java/com/acme/domain/Order.java:3 com.acme.adapters.db.OrderRow", "internal src/main/java/com/acme/adapters/db")
	wantImport(t, m, "src/main/java/com/acme/domain/Order.java:4 java.util.List", "std")
	wantImport(t, m, "src/main/java/com/acme/domain/Order.java:5 org.springframework.stereotype.Service", "third")
	wantImport(t, m, "src/main/java/com/acme/domain/Order.java:6 com.acme.domain.Money.of", "internal src/main/java/com/acme/domain")
	wantFindings(t, evaluate(m, javaPack),
		"src/main/java/com/acme/domain/Order.java:3 imports",
		"src/main/java/com/acme/domain/Order.java:5 imports",
		"src/main/java/com/acme/domain/Order.java:12 impure",
	)
	for _, p := range m.Packages {
		if p.Dir == "src/test/java/com/acme/domain" && (len(p.TestFiles) != 1 || len(p.Imports) != 0) {
			t.Errorf("test files are listed, not scanned: %+v", p)
		}
	}
}

func TestKotlinReadsThePackageHeader(t *testing.T) {
	kt := regexPack("kotlin", "package-dir", `^\s*import\s+([A-Za-z0-9_.]+)(?:\.\*)?(?:\s+as\s+\w+)?\s*$`,
		[]string{"**/src/main/**/*.kt"}, nil, nil, []string{"Instant.now"})
	m := scan(t, map[string]string{
		// the directory says nothing: the package header decides
		"src/main/kotlin/core/Order.kt": "@file:JvmName(\"O\")\n\npackage com.acme.domain\n\nimport com.acme.adapters.*\nimport kotlin.math.max\n\nval t = Instant.now()\n",
		"src/main/kotlin/infra/Db.kt":   "package com.acme.adapters\n\nclass Db\n",
	}, kt)
	wantImport(t, m, "src/main/kotlin/core/Order.kt:5 com.acme.adapters", "internal src/main/kotlin/infra")
	wantImport(t, m, "src/main/kotlin/core/Order.kt:6 kotlin.math.max", "std")
	// core/ is no conventional layer name, so configure it as the domain.
	rep := archrule.EvaluateWith(m, archrule.Layers{Domain: []string{"**/core"}, Adapters: []string{"**/infra"}},
		archrule.Rules{ImpureCalls: kt.Arch.ImpureCalls})
	wantFindings(t, rep, "src/main/kotlin/core/Order.kt:5 imports", "src/main/kotlin/core/Order.kt:8 impure")
}

func TestScalaChainedPackageClauses(t *testing.T) {
	sc := regexPack("scala", "package-dir", `^\s*import\s+([A-Za-z_][A-Za-z0-9_.]*)`,
		[]string{"**/src/main/scala/**/*.scala"}, nil, nil, nil)
	m := scan(t, map[string]string{
		"src/main/scala/acme/domain/Order.scala":  "package acme\npackage domain\n\nimport acme.adapters.Db\nimport scala.util.Try\nimport cats.effect.IO\n",
		"src/main/scala/acme/adapters/Db.scala":   "package acme.adapters\n\nobject Db\n",
		"src/main/scala/acme/domain/Money.scala":  "// header\npackage acme.domain\n",
		"src/main/scala/acme/domain/object.scala": "package acme\npackage object domain {}\n",
	}, sc)
	wantImport(t, m, "src/main/scala/acme/domain/Order.scala:4 acme.adapters.Db", "internal src/main/scala/acme/adapters")
	wantImport(t, m, "src/main/scala/acme/domain/Order.scala:5 scala.util.Try", "std")
	wantImport(t, m, "src/main/scala/acme/domain/Order.scala:6 cats.effect.IO", "third")
}

var tsPack = regexPack("typescript", "relative-path",
	`(?:import|export)\s+(?:type\s+)?(?:[^'";]*?\s+from\s+)?['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)|require\(\s*['"]([^'"]+)['"]\s*\)`,
	[]string{"**/*.ts", "**/*.js"}, []string{"**/*.test.ts"},
	[]string{"node:fs", "fs", "@nestjs/*", "express"}, []string{"Date.now", "Math.random"})

func TestTypeScriptRelativePath(t *testing.T) {
	m := scan(t, map[string]string{
		"tsconfig.json": `{
  // comments and trailing commas are allowed
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {"@adapters/*": ["src/adapters/*"], "@cfg": ["src/config/index.ts"],}, /* block */
  },
}`,
		"src/domain/order.ts": `import { Repo } from "../adapters/db";
import {
  a,
  b,
} from "../adapters/db/repo.js";
import type { Http } from "@adapters/http";
import fs from "node:fs";
import { readFile } from "fs/promises";
import { z } from "zod";
import { Controller } from "@nestjs/common";
import cfg from "@cfg";
import { money } from "./money";
import { util } from "src/shared/util";
// import { x } from "../adapters/ignored";
const lazy = await import("../adapters/lazy");
const req = require("express");
export const now = () => Date.now();
`,
		"src/domain/money.ts":       "export const money = 1;\n",
		"src/domain/order.test.ts":  "import { Repo } from '../adapters/db';\n",
		"src/adapters/db/index.ts":  "export class Repo {}\n",
		"src/adapters/db/repo.ts":   "export const a = 1, b = 2;\n",
		"src/adapters/http.ts":      "import { Order } from '../domain/order';\n",
		"src/adapters/lazy.js":      "module.exports = {};\n",
		"src/config/index.ts":       "export default {};\n",
		"src/shared/util.ts":        "export const util = 1;\n",
		"node_modules/zod/index.ts": "import fs from 'fs';\n",
	}, tsPack)
	f := "src/domain/order.ts"
	wantImport(t, m, f+":1 ../adapters/db", "internal src/adapters/db")
	wantImport(t, m, f+":5 ../adapters/db/repo.js", "internal src/adapters/db")
	wantImport(t, m, f+":6 @adapters/http", "internal src/adapters")
	wantImport(t, m, f+":7 node:fs", "std")
	wantImport(t, m, f+":8 fs/promises", "std")
	wantImport(t, m, f+":9 zod", "third")
	wantImport(t, m, f+":11 @cfg", "internal src/config")
	wantImport(t, m, f+":12 ./money", "internal src/domain")
	wantImport(t, m, f+":13 src/shared/util", "internal src/shared")
	wantImport(t, m, f+":15 ../adapters/lazy", "internal src/adapters")
	wantImport(t, m, f+":16 express", "third")
	if _, ok := imports(m)[f+":14 ../adapters/ignored"]; ok {
		t.Error("an import in a // comment line must be skipped")
	}
	wantFindings(t, evaluate(m, tsPack),
		f+":1 imports", f+":5 imports", f+":6 imports", f+":7 imports", f+":8 imports",
		f+":10 imports", f+":15 imports", f+":16 imports", f+":17 impure",
	)
}

func TestTypeScriptWithoutTsconfig(t *testing.T) {
	m := scan(t, map[string]string{
		"src/domain/a.ts":   "import x from '../../../escape';\nimport y from '../missing/thing';\nimport z from '@scope/pkg';\n",
		"tsconfig.json":     "{ not json",
		"src/missing/.keep": "",
	}, tsPack)
	wantImport(t, m, "src/domain/a.ts:1 ../../../escape", "third")
	wantImport(t, m, "src/domain/a.ts:2 ../missing/thing", "internal src/missing")
	wantImport(t, m, "src/domain/a.ts:3 @scope/pkg", "third")
}

var phpPack = regexPack("php", "namespace-psr4", `^\s*use\s+(?:function\s+|const\s+)?([A-Za-z_\\][A-Za-z0-9_\\]*)`,
	[]string{"**/*.php"}, []string{"tests/**/*.php"},
	[]string{`Illuminate\`, "PDO"}, []string{"new DateTime(", "time("})

func TestPHPNamespacePSR4(t *testing.T) {
	m := scan(t, map[string]string{
		"composer.json": `{"autoload": {"psr-4": {"App\\": "src/", "Lib\\Util\\": ["lib/util/", "lib/extra/"]}},
		                  "autoload-dev": {"psr-4": {"App\\Tests\\": "tests/"}}}`,
		"src/Domain/Order.php": `<?php
namespace App\Domain;

use App\Adapters\Db\OrderRepo;
use App\Domain\Money;
use DateTimeImmutable;
use Illuminate\Support\Str;
use Lib\Util\Strings;
use function App\Adapters\helper;
use PDO;
use Vendor\Pkg\Thing;
# use App\Adapters\Ignored;

final class Order { public function at() { return new DateTime('now'); } }
`,
		"src/Domain/Money.php":          "<?php\nnamespace App\\Domain;\n",
		"src/Adapters/Db/OrderRepo.php": "<?php\nnamespace App\\Adapters\\Db;\n",
		"lib/extra/Strings.php":         "<?php\nnamespace Lib\\Util;\n",
		"tests/Domain/OrderTest.php":    "<?php\nuse App\\Adapters\\Db\\OrderRepo;\n",
	}, phpPack)
	f := "src/Domain/Order.php"
	wantImport(t, m, f+":4 App\\Adapters\\Db\\OrderRepo", "internal src/Adapters/Db")
	wantImport(t, m, f+":5 App\\Domain\\Money", "internal src/Domain")
	wantImport(t, m, f+":6 DateTimeImmutable", "std")
	wantImport(t, m, f+":7 Illuminate\\Support\\Str", "third")
	wantImport(t, m, f+":8 Lib\\Util\\Strings", "internal lib/extra")
	wantImport(t, m, f+":9 App\\Adapters\\helper", "internal src/Adapters")
	wantImport(t, m, f+":10 PDO", "std")
	wantImport(t, m, f+":11 Vendor\\Pkg\\Thing", "third")
	wantFindings(t, evaluate(m, phpPack), f+":4 imports", f+":7 imports", f+":9 imports", f+":10 imports", f+":14 impure")
}

var cppPack = regexPack("cpp", "include-path", `^\s*#\s*include\s*[<"]([^>"]+)[>"]`,
	[]string{"**/*.cpp", "**/*.h"}, []string{"tests/**"},
	[]string{"boost/asio", "fstream", "sys/socket.h"}, []string{"std::chrono::system_clock::now", "std::rand("})

func TestCppIncludePath(t *testing.T) {
	m := scan(t, map[string]string{
		"src/domain/order.cpp": `#include "order.h"
#include "adapters/db.h"
#include <shop/adapters/http.h>
#include <vector>
#include <stdio.h>
#include <sys/socket.h>
#include <boost/asio.hpp>
#include "../adapters/cache.h"
// #include "adapters/ignored.h"
auto t = std::chrono::system_clock::now();
`,
		"src/domain/order.h":           "#pragma once\n",
		"include/adapters/db.h":        "#pragma once\n",
		"include/shop/adapters/http.h": "#pragma once\n",
		"src/adapters/cache.h":         "#pragma once\n",
		"tests/order_test.cpp":         "#include \"adapters/db.h\"\n",
	}, cppPack)
	f := "src/domain/order.cpp"
	wantImport(t, m, f+":1 order.h", "internal src/domain")
	wantImport(t, m, f+":2 adapters/db.h", "internal include/adapters")
	wantImport(t, m, f+":3 shop/adapters/http.h", "internal include/shop/adapters")
	wantImport(t, m, f+":4 vector", "std")
	wantImport(t, m, f+":5 stdio.h", "std")
	wantImport(t, m, f+":6 sys/socket.h", "std")
	wantImport(t, m, f+":7 boost/asio.hpp", "third")
	wantImport(t, m, f+":8 ../adapters/cache.h", "internal src/adapters")
	wantFindings(t, evaluate(m, cppPack), f+":2 imports", f+":3 imports", f+":6 imports", f+":7 imports", f+":8 imports", f+":10 impure")
}

var rustPack = regexPack("rust", "crate-path",
	`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:use|extern\s+crate)\s+([A-Za-z_][A-Za-z0-9_]*)`,
	[]string{"**/*.rs"}, []string{"**/tests/**/*.rs"},
	[]string{"std::fs", "tokio"}, []string{"SystemTime::now", "Instant::now"})

func TestRustCratePath(t *testing.T) {
	m := scan(t, map[string]string{
		"Cargo.toml":        "[workspace]\nmembers = [\"crates/*\"]\n\n[package]\nname = \"shop\"\n",
		"src/lib.rs":        "pub mod domain;\npub mod adapters;\n",
		"src/domain/mod.rs": "pub mod order;\npub mod money;\n",
		"src/domain/order.rs": `use crate::adapters::db::Repo;
use super::money::Money;
use self::inner::Thing;
use std::fs;
use std::collections::HashMap;
use serde::Serialize;
pub use crate::domain::{money, order};
use shop_infra::Client;
extern crate tokio;
// use crate::adapters::ignored;
fn at() { let t = SystemTime::now(); }
`,
		"src/domain/money.rs":           "pub struct Money;\n",
		"src/domain/order/inner.rs":     "pub struct Thing;\n",
		"src/adapters/mod.rs":           "pub mod db;\n",
		"src/adapters/db.rs":            "use crate::domain::order::Order;\n",
		"crates/infra/Cargo.toml":       "[package]\nname = \"shop-infra\"\nversion = \"0.1.0\"\n",
		"crates/infra/src/lib.rs":       "use crate::client::Client;\nmod client;\n",
		"crates/infra/src/client.rs":    "pub struct Client;\n",
		"tests/it.rs":                   "use shop::adapters::db::Repo;\n",
		"target/debug/build/gen/out.rs": "use crate::adapters::db::Repo;\n",
	}, rustPack)
	f := "src/domain/order.rs"
	wantImport(t, m, f+":1 crate::adapters::db::Repo", "internal src/adapters/db")
	wantImport(t, m, f+":2 super::money::Money", "internal src/domain/money")
	wantImport(t, m, f+":3 self::inner::Thing", "internal src/domain/order/inner")
	wantImport(t, m, f+":4 std::fs", "std")
	wantImport(t, m, f+":5 std::collections::HashMap", "std")
	wantImport(t, m, f+":6 serde::Serialize", "third")
	wantImport(t, m, f+":7 crate::domain", "internal src/domain")
	wantImport(t, m, f+":8 shop_infra::Client", "internal crates/infra/src")
	wantImport(t, m, f+":9 tokio", "third")
	wantImport(t, m, "crates/infra/src/lib.rs:1 crate::client::Client", "internal crates/infra/src/client")
	// modules are units: src/domain/order.rs is src/domain/order, lib.rs is src
	units := map[string]bool{}
	for _, p := range m.Packages {
		units[p.Dir] = true
	}
	for _, u := range []string{"src", "src/domain", "src/domain/order", "src/adapters/db", "tests/it"} {
		if !units[u] {
			t.Errorf("no unit %s in %v", u, units)
		}
	}
	wantFindings(t, evaluate(m, rustPack), f+":1 imports", f+":4 imports", f+":8 imports", f+":9 imports", f+":11 impure")
}

var swiftPack = regexPack("swift", "target-name",
	`^\s*(?:@\w+\s+)*(?:(?:public|package|internal|fileprivate|private)\s+)?import\s+(?:(?:typealias|struct|class|enum|protocol|let|var|func)\s+)?([A-Za-z_][A-Za-z0-9_]*)`,
	[]string{"Sources/**/*.swift"}, []string{"Tests/**/*.swift"},
	[]string{"Vapor", "FoundationNetworking"}, []string{"Date.now", "UUID()"})

func TestSwiftTargetName(t *testing.T) {
	m := scan(t, map[string]string{
		"Package.swift":                      "// swift-tools-version:6.0\n",
		"Sources/Domain/Order.swift":         "import Foundation\n@preconcurrency import Adapters\nprivate import struct Vapor.Request\nimport Collections\n\nlet at = Date.now\n",
		"Sources/Adapters/Db.swift":          "import Domain\n",
		"Tests/DomainTests/OrderTests.swift": "import XCTest\n@testable import Adapters\n",
	}, swiftPack)
	f := "Sources/Domain/Order.swift"
	wantImport(t, m, f+":1 Foundation", "std")
	wantImport(t, m, f+":2 Adapters", "internal Sources/Adapters")
	wantImport(t, m, f+":3 Vapor", "third")
	wantImport(t, m, f+":4 Collections", "third")
	wantImport(t, m, "Sources/Adapters/Db.swift:1 Domain", "internal Sources/Domain")
	wantFindings(t, evaluate(m, swiftPack), f+":2 imports", f+":3 imports", f+":6 impure")
}

var cljPack = regexPack("clojure", "dotted-module-underscore",
	`\[\s*([a-z][A-Za-z0-9_.\-]*(?:\.[A-Za-z0-9_\-]+)+)(?:\s|\])`,
	[]string{"src/**/*.clj", "src/**/*.cljc"}, []string{"test/**"},
	[]string{"next.jdbc", "clojure.java.io"}, []string{"Instant/now", "(rand"})

func TestClojureDottedModules(t *testing.T) {
	m := scan(t, map[string]string{
		"deps.edn": "{:paths [\"src\"]}\n",
		"src/shop/domain/order.clj": `(ns shop.domain.order
  (:require [shop.adapters.db :as db]
            [shop.adapters.http-client :as http]
            [shop.domain.money :as money]
            [clojure.string :as str]
            [clojure.java.io :as io]
            [next.jdbc :as jdbc]))
;; (Instant/now) in a comment is not a call
(defn at [] (Instant/now))
`,
		"src/shop/domain/money.cljc":        "(ns ^:pure shop.domain.money)\n",
		"src/shop/adapters/db.clj":          "(ns shop.adapters.db)\n",
		"src/shop/adapters/http_client.clj": "; no ns form: resolved by path\n",
		"test/shop/domain/order_test.clj":   "(ns shop.domain.order-test (:require [shop.adapters.db]))\n",
	}, cljPack)
	f := "src/shop/domain/order.clj"
	wantImport(t, m, f+":2 shop.adapters.db", "internal src/shop/adapters/db")
	wantImport(t, m, f+":3 shop.adapters.http-client", "internal src/shop/adapters/http_client")
	wantImport(t, m, f+":4 shop.domain.money", "internal src/shop/domain/money")
	wantImport(t, m, f+":5 clojure.string", "std")
	wantImport(t, m, f+":7 next.jdbc", "third")
	wantFindings(t, evaluate(m, cljPack), f+":2 imports", f+":3 imports", f+":6 imports", f+":7 imports", f+":9 impure")
}

var csPack = regexPack("csharp", "project-reference",
	`^\s*(?:global\s+)?using\s+(?:static\s+)?(?:[A-Za-z_]\w*\s*=\s*)?([A-Za-z_][\w.]*)\s*;`,
	[]string{"**/*.cs"}, []string{"tests/**/*.cs"},
	[]string{"System.IO", "Microsoft.EntityFrameworkCore"}, []string{"DateTime.UtcNow", "Guid.NewGuid"})

func TestCSharpProjectReference(t *testing.T) {
	m := scan(t, map[string]string{
		"src/Shop.Domain/Shop.Domain.csproj": `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <ProjectReference Include="..\Shop.Infrastructure\Shop.Infrastructure.csproj" />
    <ProjectReference Include="..\..\..\elsewhere\Other.csproj" />
  </ItemGroup>
</Project>`,
		"src/Shop.Domain/Orders/Order.cs": `using System;
using System.IO;
using Shop.Infrastructure.Db;
using Shop.Domain.Money;
using Microsoft.EntityFrameworkCore;
global using Json = Newtonsoft.Json;

namespace Shop.Domain.Orders;

public class Order { public DateTime At = DateTime.UtcNow; }
`,
		"src/Shop.Domain/Money/Money.cs":                     "namespace Shop.Domain.Money { public record Money; }\n",
		"src/Shop.Infrastructure/Shop.Infrastructure.csproj": "<Project Sdk=\"Microsoft.NET.Sdk\"></Project>\n",
		"src/Shop.Infrastructure/Db/Repo.cs":                 "namespace Shop.Infrastructure.Db;\nusing Shop.Domain.Orders;\n",
		"src/Shop.Infrastructure/obj/Gen.cs":                 "using Shop.Domain;\n",
		"tests/Shop.Tests/OrderTests.cs":                     "using Shop.Infrastructure.Db;\n",
	}, csPack)
	f := "src/Shop.Domain/Orders/Order.cs"
	wantImport(t, m, f+":1 System", "std")
	wantImport(t, m, f+":2 System.IO", "std")
	wantImport(t, m, f+":3 Shop.Infrastructure.Db", "internal src/Shop.Infrastructure")
	wantImport(t, m, f+":4 Shop.Domain.Money", "internal src/Shop.Domain")
	wantImport(t, m, f+":5 Microsoft.EntityFrameworkCore", "third")
	wantImport(t, m, f+":6 Newtonsoft.Json", "third")
	csproj := "src/Shop.Domain/Shop.Domain.csproj"
	wantImport(t, m, csproj+":3 Shop.Infrastructure", "internal src/Shop.Infrastructure")
	wantImport(t, m, csproj+":4 Other", "third")
	wantFindings(t, evaluate(m, csPack), f+":2 imports", f+":3 imports", f+":5 imports", f+":10 impure", csproj+":3 imports")
}

func TestScanErrors(t *testing.T) {
	root := writeTree(t, map[string]string{"a.java": "package a;\n"})
	s := importscan.New()
	bad := []pack.Pack{
		{ID: "go", Arch: pack.Arch{Parser: pack.ParserGo}},
		{ID: "x", Arch: pack.Arch{Parser: pack.ParserRegex, ImportRegex: "(", Resolver: "package-dir"}},
		{ID: "y", Arch: pack.Arch{Parser: pack.ParserRegex, ImportRegex: "import", Resolver: "nope"}},
	}
	for _, p := range bad {
		if _, err := s.Scan(context.Background(), root, p); err == nil {
			t.Errorf("pack %s: want an error", p.ID)
		}
	}
	if _, err := s.Scan(context.Background(), filepath.Join(root, "missing"), javaPack); err == nil {
		t.Error("a missing root must be an error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Scan(ctx, root, javaPack); err == nil {
		t.Error("a cancelled context must stop the walk")
	}
}

func TestModuleShape(t *testing.T) {
	m := scan(t, map[string]string{
		"src/main/java/a/A.java": "package a;\nimport b.B;\n\n",
		"src/main/java/b/B.java": "package b;\n",
	}, javaPack)
	if m.Path != "java" || m.Parser != pack.ParserRegex || m.IsGo() {
		t.Errorf("module = %q parser %q", m.Path, m.Parser)
	}
	if len(m.Packages) != 2 || m.Packages[0].Dir != "src/main/java/a" || m.Packages[0].Lines != 3 {
		t.Fatalf("packages = %+v", m.Packages)
	}
}
