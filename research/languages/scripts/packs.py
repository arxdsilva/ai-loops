import json,re,sys
L=["python","typescript","java","cpp","csharp","php","go","ruby","swift","kotlin","rust","scala","elixir","erlang","clojure"]
root=sys.argv[1]
out={}
for l in L:
    t=open(f"{root}/{l}.md").read()
    s=t.index("## 15. LOOPS")
    m=re.search(r"```json\n(.*?)\n```", t[s:], re.S)
    out[l]=json.loads(m.group(1))
json.dump(out,open("packs.json","w"),indent=1)
keys=set()
for l,p in out.items(): keys|=set(p)
std=["id","name","versions","detect","source_globs","test_globs","generated","checks","test_commands","race","coverage","post_edit","cache_env","artifacts","arch","test_jobs","config_files","sources"]
print("extra top-level keys:")
for k in sorted(keys-set(std)): print(" ",k,[l for l in L if k in out[l]])
print("missing std keys:",[(l,k) for l in L for k in std if k not in out[l]])
ck=set(); kinds={}; fo={}
for l,p in out.items():
    for c in p["checks"]:
        ck|=set(c); kinds.setdefault(c["kind"],[]).append(l); 
        if "fails_on" in c: fo.setdefault(c["fails_on"].split(":")[0],[]).append(l)
print("check keys:",sorted(ck))
print("kinds:",{k:sorted(set(v)) for k,v in kinds.items()})
print("fails_on:",{k:sorted(set(v)) for k,v in fo.items()})
for l,p in out.items():
    r=p["race"]; c=p["coverage"]
    print(f"{l:11} race={r['mode']:5} racekeys={sorted(set(r)-{'mode','flags','env','reason','substitute'})} cov={c.get('format')} path={c.get('path')} covkeys={sorted(set(c)-{'format','path','granularity'})}")
    print(f"{'':11} cache_env={p.get('cache_env')}")
ph=set()
for l,p in out.items():
    for x in re.findall(r"\{[a-z_]+\}", json.dumps(p)): ph.add((x,l))
from collections import defaultdict
pp=defaultdict(set)
for x,l in ph: pp[x].add(l)
print("placeholders:",{k:sorted(v) for k,v in pp.items()})
