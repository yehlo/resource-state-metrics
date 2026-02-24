# Resolver

A `Resolver` takes a query string and an unstructured object map and returns
`map[string]string` — a flat key/value representation of whatever the query
resolved to.

```go
type Resolver interface {
    Resolve(query string, obj map[string]interface{}) map[string]string
}
```

The contract for the returned map is:

| Result kind | Map shape |
|---|---|
| Scalar | `{query: "value"}` — exactly one entry, key is the full query string |
| Error / not found | `{query: query}` — key equals value, signals a no-op to the caller |
| List | `{"fieldParent#0": "v0", "fieldParent#1": "v1", …}` — one entry per element |
| Map | `{"k1": "v1", "k2": "v2", …}` — one entry per leaf key |

---

## Unstructured resolver

Splits the query on `.` and delegates to `unstructured.NestedFieldNoCopy`.

```
query: "spec.replicas"
obj:   {spec: {replicas: 3}}

→ NestedFieldNoCopy(obj, "spec", "replicas") = 3, found=true
→ {query: fmt.Sprintf("%v", 3)}
= {"spec.replicas": "3"}
```

**Limitations:**

- Only dot-notation paths are supported — index syntax (`field[0]`) and
  nested-map traversal beyond what `NestedFieldNoCopy` supports are not.
- Composite values (maps, slices) that `NestedFieldNoCopy` returns as a single
  `interface{}` are stringified whole (e.g. `map[k:v]`).
- When the field is not found or traversal errors, the map echoes the query
  back as both key and value: `{query: query}`. The caller detects this pattern
  to skip the metric.

---

## CEL resolver

Compiles and evaluates a CEL expression against `{o: obj}`. The result type
drives how the output map is built.

### Scalar types

`bool`, `double`, `int`, `string`, `uint` → single entry keyed by the full
query string, value is `fmt.Sprintf("%v", out.Value())`.

```
query: "o.metadata.name"
→ StringType "test-sample"
→ {"o.metadata.name": "test-sample"}
```

### Map type

`resolveMap` / `resolveMapInner` walks the CEL map recursively. Each leaf
becomes a flat key/value pair.

```
query: "o.metadata.labels"
→ MapType {app: "frontend", tier: "web"}
→ {"app": "frontend", "tier": "web"}
```

### List type — the `#N` keys

`resolveList` handles `[]interface{}` (native Go slices from the unstructured
object) and `[]ref.Val` (CEL-typed lists produced by operations like `.map()`,
`.filter()`).

`resolveListInner` emits one entry per element:

```
fieldParent = "conditions"          ← derived from the query (see below)
list        = ["Degraded", "Progressing", "Ready"]

→ {"conditions#0": "Degraded",
   "conditions#1": "Progressing",
   "conditions#2": "Ready"}
```

`fieldParent` is derived from the query by stripping the first function call
and its preceding method name, leaving the field being iterated:

```
query = "o.spec.conditions.map(c, c.type)"

  strip args at first '(':  "o.spec.conditions.map"
  strip method name:         "o.spec.conditions"
  take last segment:         "conditions"
```

This is stable for chained calls too:

```
"o.spec.items.filter(x, x.active).map(c, c.name)"
  → strip at first '(': "o.spec.items.filter"
  → strip method name:  "o.spec.items"
  → field parent:       "items"
```

---

## From query to metric — full CEL flow

Using this Bar resource and metric config as a worked example:

```yaml
# Bar resource
metadata:
  name: test-sample
spec:
  conditions:
    - {type: Degraded,    status: "False"}
    - {type: Progressing, status: "Unknown"}
    - {type: Ready,       status: "True"}
```

```yaml
# Metric config
labels:
  - name: "name"
    value: "o.metadata.name"
  - name: "type"
    value: "o.spec.conditions.map(c, c.type)"
value: "o.spec.conditions.map(c, int(c.status == 'True' ? 1 : 0))"
```

### Step 1 — `resolveLabels` (`family.go`)

Each `Label` is resolved in turn.

**`name` label — scalar:**

```
Resolve("o.metadata.name", obj)
  → StringType "test-sample"
  → {"o.metadata.name": "test-sample"}

key "o.metadata.name" found in map → scalar path
  resolvedLabelKeys   = ["name"]
  resolvedLabelValues = ["test-sample"]
```

**`type` label — list:**

```
Resolve("o.spec.conditions.map(c, c.type)", obj)
  → ListType, []ref.Val{"Degraded","Progressing","Ready"}
  → resolveList → resolveListInner
  → {"conditions#0":"Degraded", "conditions#1":"Progressing", "conditions#2":"Ready"}

key "o.spec.conditions.map(…)" NOT in map → list path
  for each "conditions#N" key: matches .+#\d+
    stored under sanitizeKey(label.Name) = "type"
  resolvedExpandedLabelSet = {"type": ["Degraded","Progressing","Ready"]}
```

### Step 2 — value resolution (`family.go` → `buildMetricString`)

```
Resolve("o.spec.conditions.map(c, int(c.status == 'True' ? 1 : 0))", obj)
  → ListType, []ref.Val{int64(0), int64(0), int64(1)}
  → resolveList → resolveListInner (int64 matches type switch)
  → {"conditions#0":"0", "conditions#1":"0", "conditions#2":"1"}

key "o.spec.conditions.map(…)" NOT found → collect by index suffix:
  "#0" → "0", "#1" → "0", "#2" → "1"
  expandedValues = ["0","0","1"]
  resolvedExpandedLabelSet["\x00"] = ["0","0","1"]   ← sentinel key
```

State before writing:

```
resolvedLabelKeys        = ["name"]
resolvedLabelValues      = ["test-sample"]
resolvedExpandedLabelSet = {
    "type": ["Degraded","Progressing","Ready"],
    "\x00": ["0","0","1"]
}
```

### Step 3 — `writeMetricSamples`

```
expandedValues = ["0","0","1"]   ← extracted from sentinel
delete(expanded, "\x00")
expanded = {"type": ["Degraded","Progressing","Ready"]}

i = 0   ← closure counter, advances with each writeMetric call
```

### Step 4 — `writeExpandedSamples`

```
labelKeys       = ["name", "type"]   ← "type" appended from expanded
seriesToGenerate = 3
slices.Sort(expanded["type"]) → ["Degraded","Progressing","Ready"]  (already sorted)

sample 0: labels = ["name","type"] / ["test-sample","Degraded"], i=0, value="0"
  → kube_customresource_bars_conditions{name="test-sample",type="Degraded",group=…} 0.000000

sample 1: labels = ["name","type"] / ["test-sample","Progressing"], i=1, value="0"
  → kube_customresource_bars_conditions{name="test-sample",type="Progressing",group=…} 0.000000

sample 2: labels = ["name","type"] / ["test-sample","Ready"], i=2, value="1"
  → kube_customresource_bars_conditions{name="test-sample",type="Ready",group=…} 1.000000
```

GVK labels (`group`, `version`, `kind`) are appended by `writeMetricTo` after
the user-defined labels.

---

## Unstructured flow (same example, different resolver)

The unstructured resolver only supports dot-notation paths, so composite
expressions like `.map()` are not available. The equivalent scalar query for a
single known condition would be:

```yaml
labels:
  - name: "name"
    value: "metadata.name"       # no "o." prefix — path starts at root
value: "metadata.labels.bar"
```

```
Resolve("metadata.name", obj)
  → NestedFieldNoCopy(obj, "metadata", "name") = "test-sample"
  → {"metadata.name": "test-sample"}

key "metadata.name" found → scalar path
  resolvedLabelKeys   = ["name"]
  resolvedLabelValues = ["test-sample"]
```

Value resolution follows the same scalar path. No `#N` keys are ever produced
by the unstructured resolver — list and map fields are stringified whole and
passed through as a single scalar value.

---

## Key invariants

- The `#N` suffix is the only signal that a resolver returned a list. Both
  `resolveLabels` (for label values) and `buildMetricString` (for metric
  values) detect it with `.+#\d+`.
- List expansion for **labels** stores values under `sanitizeKey(label.Name)`
  so the user's chosen label name is preserved in the output metric.
- List expansion for **values** uses the NUL-byte sentinel key `"\x00"` to
  carry per-sample values through to `writeMetricSamples` without polluting
  the label namespace.
- `slices.Sort` on each expansion key's values is what makes label ordering
  deterministic. Conditions defined in alphabetical order in the resource
  therefore produce correctly paired label/value expansions.
- The unstructured resolver never emits `#N` keys; it is therefore incompatible
  with list-valued labels or values and is intended for simple scalar field
  extraction only.
