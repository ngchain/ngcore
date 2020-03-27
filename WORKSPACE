load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.22.1/rules_go-v0.22.1.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.22.1/rules_go-v0.22.1.tar.gz",
    ],
    sha256 = "e6a6c016b0663e06fa5fccf1cd8152eab8aa8180c583ec20c872f4f9953a7ac5",
)

http_archive(
    name = "bazel_gazelle",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/v0.20.0/bazel-gazelle-v0.20.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.20.0/bazel-gazelle-v0.20.0.tar.gz",
    ],
    sha256 = "d8c45ee70ec39a57e7a05e5027c32b1576cc7f16d9dd37135b0eddde45cf1b10",
)

load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

git_repository(
    name = "com_google_protobuf",
    commit = "09745575a923640154bcf307fba8aedff47f240a",
    remote = "https://github.com/protocolbuffers/protobuf",
    shallow_since = "1558721209 -0700",
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "co_honnef_go_tools",
    importpath = "honnef.co/go/tools",
    sum = "h1:3JgtbtFHMiCmsznwGVTUWbgGov+pVqnlf1dEJTNAXeM=",
    version = "v0.0.1-2019.2.3",
)

go_repository(
    name = "com_github_aead_siphash",
    importpath = "github.com/aead/siphash",
    sum = "h1:FwHfE/T45KPKYuuSAKyyvE+oPWcaQ+CUmFW0bPlM+kg=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_aead_skein",
    importpath = "github.com/aead/skein",
    sum = "h1:q5TSngwXJdajCyZPQR+eKyRRgI3/ZXC/Nq1ZxZ4Zxu8=",
    version = "v0.0.0-20160722084837-9365ae6e95d2",
)

go_repository(
    name = "com_github_andreasbriese_bbloom",
    importpath = "github.com/AndreasBriese/bbloom",
    sum = "h1:HD8gA2tkByhMAwYaFAX9w2l7vxvBQ5NMoxDrkhqhtn4=",
    version = "v0.0.0-20190306092124-e2d15f34fcf9",
)

go_repository(
    name = "com_github_armon_consul_api",
    importpath = "github.com/armon/consul-api",
    sum = "h1:G1bPvciwNyF7IUmKXNt9Ak3m6u9DE1rF+RmtIkBpVdA=",
    version = "v0.0.0-20180202201655-eb2c6b5be1b6",
)

go_repository(
    name = "com_github_btcsuite_btcd",
    importpath = "github.com/btcsuite/btcd",
    sum = "h1:Ik4hyJqN8Jfyv3S4AGBOmyouMsYE3EdYODkMbQjwPGw=",
    version = "v0.20.1-beta",
)

go_repository(
    name = "com_github_btcsuite_btclog",
    importpath = "github.com/btcsuite/btclog",
    sum = "h1:bAs4lUbRJpnnkd9VhRV3jjAVU7DJVjMaK+IsvSeZvFo=",
    version = "v0.0.0-20170628155309-84c8d2346e9f",
)

go_repository(
    name = "com_github_btcsuite_btcutil",
    importpath = "github.com/btcsuite/btcutil",
    sum = "h1:yJzD/yFppdVCf6ApMkVy8cUxV0XrxdP9rVf6D87/Mng=",
    version = "v0.0.0-20190425235716-9e5f4b9a998d",
)

go_repository(
    name = "com_github_btcsuite_go_socks",
    importpath = "github.com/btcsuite/go-socks",
    sum = "h1:R/opQEbFEy9JGkIguV40SvRY1uliPX8ifOvi6ICsFCw=",
    version = "v0.0.0-20170105172521-4720035b7bfd",
)

go_repository(
    name = "com_github_btcsuite_goleveldb",
    importpath = "github.com/btcsuite/goleveldb",
    sum = "h1:qdGvebPBDuYDPGi1WCPjy1tGyMpmDK8IEapSsszn7HE=",
    version = "v0.0.0-20160330041536-7834afc9e8cd",
)

go_repository(
    name = "com_github_btcsuite_snappy_go",
    importpath = "github.com/btcsuite/snappy-go",
    sum = "h1:ZA/jbKoGcVAnER6pCHPEkGdZOV7U1oLUedErBHCUMs0=",
    version = "v0.0.0-20151229074030-0bdef8d06723",
)

go_repository(
    name = "com_github_btcsuite_websocket",
    importpath = "github.com/btcsuite/websocket",
    sum = "h1:R8vQdOQdZ9Y3SkEwmHoWBmX1DNXhXZqlTpq6s4tyJGc=",
    version = "v0.0.0-20150119174127-31079b680792",
)

go_repository(
    name = "com_github_btcsuite_winsvc",
    importpath = "github.com/btcsuite/winsvc",
    sum = "h1:J9B4L7e3oqhXOcm+2IuNApwzQec85lE+QaikUcCs+dk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_burntsushi_toml",
    importpath = "github.com/BurntSushi/toml",
    sum = "h1:WXkYYl6Yr3qBf1K79EBnL4mak0OimBfB0XUf9Vl28OQ=",
    version = "v0.3.1",
)

go_repository(
    name = "com_github_cbergoon_merkletree",
    importpath = "github.com/cbergoon/merkletree",
    sum = "h1:Bttqr3OuoiZEo4ed1L7fTasHka9II+BF9fhBfbNEEoQ=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_client9_misspell",
    importpath = "github.com/client9/misspell",
    sum = "h1:ta993UF76GwbvJcIo3Y68y/M3WxlpEHPWIGDkJYwzJI=",
    version = "v0.3.4",
)

go_repository(
    name = "com_github_coreos_etcd",
    importpath = "github.com/coreos/etcd",
    sum = "h1:jFneRYjIvLMLhDLCzuTuU4rSJUjRplcJQ7pD7MnhC04=",
    version = "v3.3.10+incompatible",
)

go_repository(
    name = "com_github_coreos_go_etcd",
    importpath = "github.com/coreos/go-etcd",
    sum = "h1:bXhRBIXoTm9BYHS3gE0TtQuyNZyeEMux2sDi4oo5YOo=",
    version = "v2.0.0+incompatible",
)

go_repository(
    name = "com_github_coreos_go_semver",
    importpath = "github.com/coreos/go-semver",
    sum = "h1:wkHLiw0WNATZnSG7epLsujiMCgPAc9xhjJ4tgnAxmfM=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_cpuguy83_go_md2man",
    importpath = "github.com/cpuguy83/go-md2man",
    sum = "h1:BSKMNlYxDvnunlTymqtgONjNnaRV1sTpcovwwjF22jk=",
    version = "v1.0.10",
)

go_repository(
    name = "com_github_davecgh_go_spew",
    importpath = "github.com/davecgh/go-spew",
    sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_davidlazar_go_crypto",
    importpath = "github.com/davidlazar/go-crypto",
    sum = "h1:6xT9KW8zLC5IlbaIF5Q7JNieBoACT7iW0YTxQHR0in0=",
    version = "v0.0.0-20170701192655-dcfb0a7ac018",
)

go_repository(
    name = "com_github_dchest_blake256",
    importpath = "github.com/dchest/blake256",
    sum = "h1:4AuEhGPT/3TTKFhTfBpZ8hgZE7wJpawcYaEawwsbtqM=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_dgraph_io_badger",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/dgraph-io/badger",
    sum = "h1:DshxFxZWXUcO0xX476VJC07Xsr6ZCBVRHKZ93Oh7Evo=",
    version = "v1.6.0",
)

go_repository(
    name = "com_github_dgryski_go_farm",
    importpath = "github.com/dgryski/go-farm",
    sum = "h1:tdlZCpZ/P9DhczCTSixgIKmwPv6+wP5DGjqLYw5SUiA=",
    version = "v0.0.0-20190423205320-6a90982ecee2",
)

go_repository(
    name = "com_github_dustin_go_humanize",
    importpath = "github.com/dustin/go-humanize",
    sum = "h1:VSnTsYCnlFHaM2/igO1h6X3HA71jcobQuxemgkq4zYo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_fsnotify_fsnotify",
    importpath = "github.com/fsnotify/fsnotify",
    sum = "h1:IXs+QLmnXW2CcXuY+8Mzv/fWEsPGWxqefPtCP5CnV9I=",
    version = "v1.4.7",
)

go_repository(
    name = "com_github_go_check_check",
    importpath = "github.com/go-check/check",
    sum = "h1:0gkP6mzaMqkmpcJYCFOLkIBwI7xFExG03bbkOkCvUPI=",
    version = "v0.0.0-20180628173108-788fd7840127",
)

go_repository(
    name = "com_github_gogo_protobuf",
    importpath = "github.com/gogo/protobuf",
    sum = "h1:DqDEcV5aeaTmdFBePNpYsp3FlcVH/2ISVVM9Qf8PSls=",
    version = "v1.3.1",
)

go_repository(
    name = "com_github_golang_glog",
    importpath = "github.com/golang/glog",
    sum = "h1:VKtxabqXZkF25pY9ekfRL6a582T4P37/31XEstQ5p58=",
    version = "v0.0.0-20160126235308-23def4e6c14b",
)

go_repository(
    name = "com_github_golang_groupcache",
    importpath = "github.com/golang/groupcache",
    sum = "h1:ZgQEtGgCBiWRM39fZuwSd1LwSqqSW0hOdXCYYDX0R3I=",
    version = "v0.0.0-20190702054246-869f871628b6",
)

go_repository(
    name = "com_github_golang_mock",
    importpath = "github.com/golang/mock",
    sum = "h1:G5FRp8JnTd7RQH5kemVNlMeyXQAztQ3mOWV95KxsXH8=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_golang_protobuf",
    importpath = "github.com/golang/protobuf",
    sum = "h1:F768QJ1E9tib+q5Sc8MkdJi1RxLTbRcTf8LJV56aRls=",
    version = "v1.3.5",
)

go_repository(
    name = "com_github_golang_snappy",
    importpath = "github.com/golang/snappy",
    sum = "h1:Qgr9rKW7uDUkrbSmQeiDsGa8SjGyCOGtuasMWwvp2P4=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_google_go_cmp",
    importpath = "github.com/google/go-cmp",
    sum = "h1:crn/baboCvb5fXaQ0IJ1SGTsTVrWpDsCWC8EGETZijY=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_google_uuid",
    importpath = "github.com/google/uuid",
    sum = "h1:Gkbcsh/GbpXz7lPftLA3P6TYMwjCLYm83jiFQZF/3gY=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_gorilla_rpc",
    importpath = "github.com/gorilla/rpc",
    sum = "h1:WvvdC2lNeT1SP32zrIce5l0ECBfbAlmrmSBsuc57wfk=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_gorilla_websocket",
    importpath = "github.com/gorilla/websocket",
    sum = "h1:q7AeDBpnBk8AogcD4DSag/Ukw/KV+YhzLj2bP5HvKCM=",
    version = "v1.4.1",
)

go_repository(
    name = "com_github_gxed_hashland_keccakpg",
    importpath = "github.com/gxed/hashland/keccakpg",
    sum = "h1:wrk3uMNaMxbXiHibbPO4S0ymqJMm41WiudyFSs7UnsU=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_gxed_hashland_murmur3",
    importpath = "github.com/gxed/hashland/murmur3",
    sum = "h1:SheiaIt0sda5K+8FLz952/1iWS9zrnKsEJaOJu4ZbSc=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_hashicorp_errwrap",
    importpath = "github.com/hashicorp/errwrap",
    sum = "h1:hLrqtEDnRye3+sgx6z4qVLNuviH3MR5aQ0ykNJa/UYA=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_multierror",
    importpath = "github.com/hashicorp/go-multierror",
    sum = "h1:iVjPR7a6H0tWELX5NxNe7bYopibicUzc7uPribsnS6o=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_golang_lru",
    importpath = "github.com/hashicorp/golang-lru",
    sum = "h1:YDjusn29QI/Das2iO9M0BHnIbxPeyuCHsjMW+lJfyTc=",
    version = "v0.5.4",
)

go_repository(
    name = "com_github_hashicorp_hcl",
    importpath = "github.com/hashicorp/hcl",
    sum = "h1:0Anlzjpi4vEasTeNFn2mLJgTSwt0+6sfsiTG8qcWGx4=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hpcloud_tail",
    importpath = "github.com/hpcloud/tail",
    sum = "h1:nfCOvKYfkgYP8hkirhJocXT2+zOD8yUNjXaWfTlyFKI=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_huin_goupnp",
    importpath = "github.com/huin/goupnp",
    sum = "h1:wg75sLpL6DZqwHQN6E1Cfk6mtfzS45z8OV+ic+DtHRo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_huin_goutil",
    importpath = "github.com/huin/goutil",
    sum = "h1:vlNjIqmUZ9CMAWsbURYl3a6wZbw7q5RHVvlXTNS/Bs8=",
    version = "v0.0.0-20170803182201-1ca381bf3150",
)

go_repository(
    name = "com_github_inconshreveable_mousetrap",
    importpath = "github.com/inconshreveable/mousetrap",
    sum = "h1:Z8tu5sraLXCXIcARxBp/8cbvlwVa7Z1NHg9XEKhtSvM=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_ipfs_go_cid",
    importpath = "github.com/ipfs/go-cid",
    sum = "h1:o0Ix8e/ql7Zb5UVUJEUfjsWCIY8t48++9lR8qi6oiJU=",
    version = "v0.0.5",
)

go_repository(
    name = "com_github_ipfs_go_datastore",
    importpath = "github.com/ipfs/go-datastore",
    sum = "h1:rjvQ9+muFaJ+QZ7dN5B1MSDNQ0JVZKkkES/rMZmA8X8=",
    version = "v0.4.4",
)

go_repository(
    name = "com_github_ipfs_go_detect_race",
    importpath = "github.com/ipfs/go-detect-race",
    sum = "h1:qX/xay2W3E4Q1U7d9lNs1sU9nvguX0a7319XbyQ6cOk=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_ipfs_go_ds_badger",
    importpath = "github.com/ipfs/go-ds-badger",
    sum = "h1:RsC9DDlwFhFdfT+s2PeC8joxbSp2YMufK8w/RBOxKtk=",
    version = "v0.2.1",
)

go_repository(
    name = "com_github_ipfs_go_ds_leveldb",
    importpath = "github.com/ipfs/go-ds-leveldb",
    sum = "h1:zaoLcP8zs4Aj9k8fA4cowyOyNBIvy9Dnt6hf7mHRY7s=",
    version = "v0.4.1",
)

go_repository(
    name = "com_github_ipfs_go_ipfs_delay",
    importpath = "github.com/ipfs/go-ipfs-delay",
    sum = "h1:NAviDvJ0WXgD+yiL2Rj35AmnfgI11+pHXbdciD917U0=",
    version = "v0.0.0-20181109222059-70721b86a9a8",
)

go_repository(
    name = "com_github_ipfs_go_ipfs_util",
    importpath = "github.com/ipfs/go-ipfs-util",
    sum = "h1:Wz9bL2wB2YBJqggkA4dD7oSmqB4cAnpNbGrlHJulv50=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_ipfs_go_log",
    importpath = "github.com/ipfs/go-log",
    sum = "h1:s19ZwJxH8rPWzypjcDpqPLIyV7BnbLqvpli3iZoqYK0=",
    version = "v1.0.2",
)

go_repository(
    name = "com_github_ipfs_go_log_v2",
    importpath = "github.com/ipfs/go-log/v2",
    sum = "h1:xguurydRdfKMJjKyxNXNU8lYP0VZH1NUwJRwUorjuEw=",
    version = "v2.0.2",
)

go_repository(
    name = "com_github_ipfs_go_todocounter",
    importpath = "github.com/ipfs/go-todocounter",
    sum = "h1:9UBngSQhylg2UDcxSAtpkT+rEWFr26hDPXVStE8LFyc=",
    version = "v0.0.2",
)

go_repository(
    name = "com_github_jackpal_gateway",
    importpath = "github.com/jackpal/gateway",
    sum = "h1:qzXWUJfuMdlLMtt0a3Dgt+xkWQiA5itDEITVJtuSwMc=",
    version = "v1.0.5",
)

go_repository(
    name = "com_github_jackpal_go_nat_pmp",
    importpath = "github.com/jackpal/go-nat-pmp",
    sum = "h1:i0LektDkO1QlrTm/cSuP+PyBCDnYvjPLGl4LdWEMiaA=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_jbenet_go_cienv",
    importpath = "github.com/jbenet/go-cienv",
    sum = "h1:Vc/s0QbQtoxX8MwwSLWWh+xNNZvM3Lw7NsTcHrvvhMc=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_jbenet_go_temp_err_catcher",
    importpath = "github.com/jbenet/go-temp-err-catcher",
    sum = "h1:vhC1OXXiT9R2pczegwz6moDvuRpggaroAXhPIseh57A=",
    version = "v0.0.0-20150120210811-aac704a3f4f2",
)

go_repository(
    name = "com_github_jbenet_goprocess",
    importpath = "github.com/jbenet/goprocess",
    sum = "h1:YKyIEECS/XvcfHtBzxtjBBbWK+MbvA6dG8ASiqwvr10=",
    version = "v0.1.3",
)

go_repository(
    name = "com_github_jessevdk_go_flags",
    importpath = "github.com/jessevdk/go-flags",
    sum = "h1:4IU2WS7AumrZ/40jfhf4QVDMsQwqA7VEHozFRrGARJA=",
    version = "v1.4.0",
)

go_repository(
    name = "com_github_jrick_logrotate",
    importpath = "github.com/jrick/logrotate",
    sum = "h1:lQ1bL/n9mBNeIXoTUoYRlK4dHuNJVofX9oWqBtPnSzI=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_jzelinskie_whirlpool",
    importpath = "github.com/jzelinskie/whirlpool",
    sum = "h1:RyOL4+OIUc6u5ac2LclitlZvFES6k+sg18fBMfxFUUs=",
    version = "v0.0.0-20170603002051-c19460b8caa6",
)

go_repository(
    name = "com_github_kami_zh_go_capturer",
    importpath = "github.com/kami-zh/go-capturer",
    sum = "h1:cVtBfNW5XTHiKQe7jDaDBSh/EVM4XLPutLAGboIXuM0=",
    version = "v0.0.0-20171211120116-e492ea43421d",
)

go_repository(
    name = "com_github_kisielk_errcheck",
    importpath = "github.com/kisielk/errcheck",
    sum = "h1:reN85Pxc5larApoH1keMBiu2GWtPqXQ1nc9gx+jOU+E=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_kisielk_gotool",
    importpath = "github.com/kisielk/gotool",
    sum = "h1:AV2c/EiW3KqPNT9ZKl07ehoAGi4C5/01Cfbblndcapg=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_kkdai_bstream",
    importpath = "github.com/kkdai/bstream",
    sum = "h1:FOOIBWrEkLgmlgGfMuZT83xIwfPDxEI2OHu6xUmJMFE=",
    version = "v0.0.0-20161212061736-f391b8402d23",
)

go_repository(
    name = "com_github_konsorten_go_windows_terminal_sequences",
    importpath = "github.com/konsorten/go-windows-terminal-sequences",
    sum = "h1:mweAR1A6xJ3oS2pRaGiHgQ4OO8tzTaLawm8vnODuwDk=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_koron_go_ssdp",
    importpath = "github.com/koron/go-ssdp",
    sum = "h1:68u9r4wEvL3gYg2jvAOgROwZ3H+Y3hIDk4tbbmIjcYQ=",
    version = "v0.0.0-20191105050749-2e1c40ed0b5d",
)

go_repository(
    name = "com_github_kr_pretty",
    importpath = "github.com/kr/pretty",
    sum = "h1:L/CwN0zerZDmRFUapSPitk6f+Q3+0za1rQkzVuMiMFI=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_kr_pty",
    importpath = "github.com/kr/pty",
    sum = "h1:VkoXIwSboBpnk99O/KFauAEILuNHv5DVFKZMBN/gUgw=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_kr_text",
    importpath = "github.com/kr/text",
    sum = "h1:45sCR5RtlFHMR4UwH9sdQ5TC8v0qDQCHnXt+kaKSTVE=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_kubuxu_go_os_helper",
    importpath = "github.com/Kubuxu/go-os-helper",
    sum = "h1:EJiD2VUQyh5A9hWJLmc6iWg6yIcJ7jpBcwC8GMGXfDk=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_libp2p_go_addr_util",
    importpath = "github.com/libp2p/go-addr-util",
    sum = "h1:TpTQm9cXVRVSKsYbgQ7GKc3KbbHVTnbostgGaDEP+88=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_libp2p_go_buffer_pool",
    importpath = "github.com/libp2p/go-buffer-pool",
    sum = "h1:QNK2iAFa8gjAe1SPz6mHSMuCcjs+X1wlHzeOSqcmlfs=",
    version = "v0.0.2",
)

go_repository(
    name = "com_github_libp2p_go_conn_security_multistream",
    importpath = "github.com/libp2p/go-conn-security-multistream",
    sum = "h1:aqGmto+ttL/uJgX0JtQI0tD21CIEy5eYd1Hlp0juHY0=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_libp2p_go_eventbus",
    importpath = "github.com/libp2p/go-eventbus",
    sum = "h1:mlawomSAjjkk97QnYiEmHsLu7E136+2oCWSHRUvMfzQ=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_libp2p_go_flow_metrics",
    importpath = "github.com/libp2p/go-flow-metrics",
    sum = "h1:8tAs/hSdNvUiLgtlSy3mxwxWP4I9y/jlkPFT7epKdeM=",
    version = "v0.0.3",
)

go_repository(
    name = "com_github_libp2p_go_libp2p",
    importpath = "github.com/libp2p/go-libp2p",
    sum = "h1:EFArryT9N7AVA70LCcOh8zxsW+FeDnxwcpWQx9k7+GM=",
    version = "v0.6.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_autonat",
    importpath = "github.com/libp2p/go-libp2p-autonat",
    sum = "h1:WLBZcIRsjZlWdAZj9CiBSvU2wQXoUOiS1Zk1tM7DTJI=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_blankhost",
    importpath = "github.com/libp2p/go-libp2p-blankhost",
    sum = "h1:I96SWjR4rK9irDHcHq3XHN6hawCRTPUADzkJacgZLvk=",
    version = "v0.1.4",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_circuit",
    importpath = "github.com/libp2p/go-libp2p-circuit",
    sum = "h1:Phzbmrg3BkVzbqd4ZZ149JxCuUWu2wZcXf/Kr6hZJj8=",
    version = "v0.1.4",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_core",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/libp2p/go-libp2p-core",
    sum = "h1:FBQ1fpq2Fo/ClyjojVJ5AKXlKhvNc/B6U0O+7AN1ffE=",
    version = "v0.5.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_crypto",
    importpath = "github.com/libp2p/go-libp2p-crypto",
    sum = "h1:k9MFy+o2zGDNGsaoZl0MA3iZ75qXxr9OOoAZF+sD5OQ=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_discovery",
    importpath = "github.com/libp2p/go-libp2p-discovery",
    sum = "h1:1p3YSOq7VsgaL+xVHPi8XAmtGyas6D2J6rWBEfz/aiY=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_kad_dht",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/libp2p/go-libp2p-kad-dht",
    sum = "h1:SzsgS12DGegFw5xU9zgYuB7negj9iWALp6fFdtDcGc4=",
    version = "v0.5.1",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_kbucket",
    importpath = "github.com/libp2p/go-libp2p-kbucket",
    sum = "h1:XtNfN4WUy0cfeJoJgWCf1lor4Pp3kBkFJ9vQ+Zs+VUM=",
    version = "v0.2.3",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_loggables",
    importpath = "github.com/libp2p/go-libp2p-loggables",
    sum = "h1:h3w8QFfCt2UJl/0/NW4K829HX/0S4KD31PQ7m8UXXO8=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_mplex",
    importpath = "github.com/libp2p/go-libp2p-mplex",
    sum = "h1:+Ld7YDAfVERQ0E+qqjE7o6fHwKuM0SqTzYiwN1lVVSA=",
    version = "v0.2.2",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_nat",
    importpath = "github.com/libp2p/go-libp2p-nat",
    sum = "h1:/mH8pXFVKleflDL1YwqMg27W9GD8kjEx7NY0P6eGc98=",
    version = "v0.0.5",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_netutil",
    importpath = "github.com/libp2p/go-libp2p-netutil",
    sum = "h1:zscYDNVEcGxyUpMd0JReUZTrpMfia8PmLKcKF72EAMQ=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_peer",
    importpath = "github.com/libp2p/go-libp2p-peer",
    sum = "h1:EQ8kMjaCUwt/Y5uLgjT8iY2qg0mGUT0N1zUjer50DsY=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_peerstore",
    importpath = "github.com/libp2p/go-libp2p-peerstore",
    sum = "h1:XcgJhI8WyUOCbHyRLNEX5542YNj8hnLSJ2G1InRjDhk=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_pnet",
    importpath = "github.com/libp2p/go-libp2p-pnet",
    sum = "h1:J6htxttBipJujEjz1y0a5+eYoiPcFHhSYHH6na5f0/k=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_record",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/libp2p/go-libp2p-record",
    sum = "h1:M50VKzWnmUrk/M5/Dz99qO9Xh4vs8ijsK+7HkJvRP+0=",
    version = "v0.1.2",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_routing",
    importpath = "github.com/libp2p/go-libp2p-routing",
    sum = "h1:hFnj3WR3E2tOcKaGpyzfP4gvFZ3t8JkQmbapN0Ct+oU=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_secio",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/libp2p/go-libp2p-secio",
    sum = "h1:eNWbJTdyPA7NxhP7J3c5lT97DC5d+u+IldkgCYFTPVA=",
    version = "v0.2.1",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_swarm",
    importpath = "github.com/libp2p/go-libp2p-swarm",
    sum = "h1:T4hUpgEs2r371PweU3DuH7EOmBIdTBCwWs+FLcgx3bQ=",
    version = "v0.2.2",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_testing",
    importpath = "github.com/libp2p/go-libp2p-testing",
    sum = "h1:U03z3HnGI7Ni8Xx6ONVZvUFOAzWYmolWf5W5jAOPNmU=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_transport_upgrader",
    importpath = "github.com/libp2p/go-libp2p-transport-upgrader",
    sum = "h1:5EhPgQhXZNyfL22ERZTUoVp9UVVbNowWNVtELQaKCHk=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_yamux",
    importpath = "github.com/libp2p/go-libp2p-yamux",
    sum = "h1:eGvbqWqWY9S5lrpe2gA0UCOLCdzCgYSAR3vo/xCsNQg=",
    version = "v0.2.2",
)

go_repository(
    name = "com_github_libp2p_go_maddr_filter",
    importpath = "github.com/libp2p/go-maddr-filter",
    sum = "h1:CW3AgbMO6vUvT4kf87y4N+0P8KUl2aqLYhrGyDUbLSg=",
    version = "v0.0.5",
)

go_repository(
    name = "com_github_libp2p_go_mplex",
    importpath = "github.com/libp2p/go-mplex",
    sum = "h1:huPH/GGRJzmsHR9IZJJsrSwIM5YE2gL4ssgl1YWb/ps=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_libp2p_go_msgio",
    importpath = "github.com/libp2p/go-msgio",
    sum = "h1:agEFehY3zWJFUHK6SEMR7UYmk2z6kC3oeCM7ybLhguA=",
    version = "v0.0.4",
)

go_repository(
    name = "com_github_libp2p_go_nat",
    importpath = "github.com/libp2p/go-nat",
    sum = "h1:KbizNnq8YIf7+Hn7+VFL/xE0eDrkPru2zIO9NMwL8UQ=",
    version = "v0.0.4",
)

go_repository(
    name = "com_github_libp2p_go_openssl",
    importpath = "github.com/libp2p/go-openssl",
    sum = "h1:d27YZvLoTyMhIN4njrkr8zMDOM4lfpHIp6A+TK9fovg=",
    version = "v0.0.4",
)

go_repository(
    name = "com_github_libp2p_go_reuseport",
    importpath = "github.com/libp2p/go-reuseport",
    sum = "h1:7PhkfH73VXfPJYKQ6JwS5I/eVcoyYi9IMNGc6FWpFLw=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_libp2p_go_reuseport_transport",
    importpath = "github.com/libp2p/go-reuseport-transport",
    sum = "h1:WglMwyXyBu61CMkjCCtnmqNqnjib0GIEjMiHTwR/KN4=",
    version = "v0.0.2",
)

go_repository(
    name = "com_github_libp2p_go_stream_muxer",
    importpath = "github.com/libp2p/go-stream-muxer",
    sum = "h1:Ce6e2Pyu+b5MC1k3eeFtAax0pW4gc6MosYSLV05UeLw=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_libp2p_go_stream_muxer_multistream",
    importpath = "github.com/libp2p/go-stream-muxer-multistream",
    sum = "h1:714bRJ4Zy9mdhyTLJ+ZKiROmAFwUHpeRidG+q7LTQOg=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_tcp_transport",
    importpath = "github.com/libp2p/go-tcp-transport",
    sum = "h1:yGlqURmqgNA2fvzjSgZNlHcsd/IulAnKM8Ncu+vlqnw=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_libp2p_go_ws_transport",
    importpath = "github.com/libp2p/go-ws-transport",
    sum = "h1:MJCw2OrPA9+76YNRvdo1wMnSOxb9Bivj6sVFY1Xrj6w=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_libp2p_go_yamux",
    importpath = "github.com/libp2p/go-yamux",
    sum = "h1:FsYzT16Wq2XqUGJsBbOxoz9g+dFklvNi7jN6YFPfl7U=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_magiconair_properties",
    importpath = "github.com/magiconair/properties",
    sum = "h1:LLgXmsheXeRoUOBOjtwPQCWIYqM/LU1ayDtDePerRcY=",
    version = "v1.8.0",
)

go_repository(
    name = "com_github_mailru_easyjson",
    importpath = "github.com/mailru/easyjson",
    sum = "h1:2gxZ0XQIU/5z3Z3bUBu+FXuk2pFbkN6tcwi/pjyaDic=",
    version = "v0.0.0-20180823135443-60711f1a8329",
)

go_repository(
    name = "com_github_mattn_go_colorable",
    importpath = "github.com/mattn/go-colorable",
    sum = "h1:G1f5SKeVxmagw/IyvzvtZE4Gybcc4Tr1tf7I8z0XgOg=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_mattn_go_isatty",
    importpath = "github.com/mattn/go-isatty",
    sum = "h1:tHXDdz1cpzGaovsTB+TVB8q90WEokoVmfMqoVcrLUgw=",
    version = "v0.0.5",
)

go_repository(
    name = "com_github_mgutz_ansi",
    importpath = "github.com/mgutz/ansi",
    sum = "h1:j7+1HpAFS1zy5+Q4qx1fWh90gTKwiN4QCGoY9TWyyO4=",
    version = "v0.0.0-20170206155736-9520e82c474b",
)

go_repository(
    name = "com_github_miekg_dns",
    importpath = "github.com/miekg/dns",
    sum = "h1:WMhc1ik4LNkTg8U9l3hI1LvxKmIL+f1+WV/SZtCbDDA=",
    version = "v1.1.12",
)

go_repository(
    name = "com_github_minio_blake2b_simd",
    importpath = "github.com/minio/blake2b-simd",
    sum = "h1:lYpkrQH5ajf0OXOcUbGjvZxxijuBwbbmlSxLiuofa+g=",
    version = "v0.0.0-20160723061019-3f5f724cb5b1",
)

go_repository(
    name = "com_github_minio_sha256_simd",
    importpath = "github.com/minio/sha256-simd",
    sum = "h1:5QHSlgo3nt5yKOJrC7W8w7X+NFl8cMPZm96iu8kKUJU=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_mitchellh_go_homedir",
    importpath = "github.com/mitchellh/go-homedir",
    sum = "h1:lukF9ziXFxDFPkA1vsr5zpc1XuPDn/wFntq5mG+4E0Y=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_mitchellh_mapstructure",
    importpath = "github.com/mitchellh/mapstructure",
    sum = "h1:fmNYVwqnSfB9mZU6OS2O6GsXM+wcskZDuKQzvN1EDeE=",
    version = "v1.1.2",
)

go_repository(
    name = "com_github_mr_tron_base58",
    importpath = "github.com/mr-tron/base58",
    sum = "h1:v+sk57XuaCKGXpWtVBX8YJzO7hMGx4Aajh4TQbdEFdc=",
    version = "v1.1.3",
)

go_repository(
    name = "com_github_multiformats_go_base32",
    importpath = "github.com/multiformats/go-base32",
    sum = "h1:tw5+NhuwaOjJCC5Pp82QuXbrmLzWg7uxlMFp8Nq/kkI=",
    version = "v0.0.3",
)

go_repository(
    name = "com_github_multiformats_go_multiaddr",
    importpath = "github.com/multiformats/go-multiaddr",
    sum = "h1:SgG/cw5vqyB5QQe5FPe2TqggU9WtrA9X4nZw7LlVqOI=",
    version = "v0.2.1",
)

go_repository(
    name = "com_github_multiformats_go_multiaddr_dns",
    importpath = "github.com/multiformats/go-multiaddr-dns",
    sum = "h1:YWJoIDwLePniH7OU5hBnDZV6SWuvJqJ0YtN6pLeH9zA=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_multiformats_go_multiaddr_fmt",
    importpath = "github.com/multiformats/go-multiaddr-fmt",
    sum = "h1:WLEFClPycPkp4fnIzoFoV9FVd49/eQsuaL3/CWe167E=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_multiformats_go_multiaddr_net",
    importpath = "github.com/multiformats/go-multiaddr-net",
    sum = "h1:P7zcBH9FRETdPkDrylcXVjQLQ2t1JQtNItZULWNWgeg=",
    version = "v0.1.2",
)

go_repository(
    name = "com_github_multiformats_go_multibase",
    importpath = "github.com/multiformats/go-multibase",
    sum = "h1:PN9/v21eLywrFWdFNsFKaU04kLJzuYzmrJR+ubhT9qA=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_multiformats_go_multihash",
    importpath = "github.com/multiformats/go-multihash",
    sum = "h1:06x+mk/zj1FoMsgNejLpy6QTvJqlSt/BhLEy87zidlc=",
    version = "v0.0.13",
)

go_repository(
    name = "com_github_multiformats_go_multistream",
    importpath = "github.com/multiformats/go-multistream",
    sum = "h1:JlAdpIFhBhGRLxe9W6Om0w++Gd6KMWoFPZL/dEnm9nI=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_multiformats_go_varint",
    importpath = "github.com/multiformats/go-varint",
    sum = "h1:XVZwSo04Cs3j/jS0uAEPpT3JY6DzMcVLLoWOSnCxOjg=",
    version = "v0.0.5",
)

go_repository(
    name = "com_github_nebulouslabs_fastrand",
    importpath = "github.com/NebulousLabs/fastrand",
    sum = "h1:n+DcnTNkQnHlwpsrHoQtkrJIO7CBx029fw6oR4vIob4=",
    version = "v0.0.0-20181203155948-6fb6489aac4e",
)

go_repository(
    name = "com_github_ngin_network_cryptonight_go",
    importpath = "github.com/ngin-network/cryptonight-go",
    sum = "h1:8RYF2rjrDkWAOfdwCPB+EnzLmUPKm5pDD3WqTwffQL4=",
    version = "v0.0.0-20190709123010-828e5b02ddcb",
)

go_repository(
    name = "com_github_onsi_ginkgo",
    importpath = "github.com/onsi/ginkgo",
    sum = "h1:Iw5WCbBcaAAd0fpRb1c9r5YCylv4XDoCSigm1zLevwU=",
    version = "v1.12.0",
)

go_repository(
    name = "com_github_onsi_gomega",
    importpath = "github.com/onsi/gomega",
    sum = "h1:R1uwffexN6Pr340GtYRIdZmAiN4J+iw6WG4wog1DUXg=",
    version = "v1.9.0",
)

go_repository(
    name = "com_github_opentracing_opentracing_go",
    importpath = "github.com/opentracing/opentracing-go",
    sum = "h1:pWlfV3Bxv7k65HYwkikxat0+s3pV4bsqf19k25Ur8rU=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_pelletier_go_toml",
    importpath = "github.com/pelletier/go-toml",
    sum = "h1:T5zMGML61Wp+FlcbWjRDT7yAxhJNAiPPLOFECq181zc=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_phoreproject_go_x11",
    importpath = "github.com/phoreproject/go-x11",
    sum = "h1:hTWt5IfyljBJQEb+ftE7NEBvan+rFKJGLTAF7Po15RE=",
    version = "v0.0.0-20190107235645-df0f52b15c3a",
)

go_repository(
    name = "com_github_pkg_errors",
    importpath = "github.com/pkg/errors",
    sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
    version = "v0.9.1",
)

go_repository(
    name = "com_github_pmezard_go_difflib",
    importpath = "github.com/pmezard/go-difflib",
    sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_russross_blackfriday",
    importpath = "github.com/russross/blackfriday",
    sum = "h1:HyvC0ARfnZBqnXwABFeSZHpKvJHJJfPz81GNueLj0oo=",
    version = "v1.5.2",
)

go_repository(
    name = "com_github_sirupsen_logrus",
    importpath = "github.com/sirupsen/logrus",
    sum = "h1:juTguoYk5qI21pwyTXY3B3Y5cOTH3ZUyZCg1v/mihuo=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_smola_gocompat",
    importpath = "github.com/smola/gocompat",
    sum = "h1:6b1oIMlUXIpz//VKEDzPVBK8KG7beVwmHIUEBIs/Pns=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_spacemonkeygo_openssl",
    importpath = "github.com/spacemonkeygo/openssl",
    sum = "h1:/eS3yfGjQKG+9kayBkj0ip1BGhq6zJ3eaVksphxAaek=",
    version = "v0.0.0-20181017203307-c2dcc5cca94a",
)

go_repository(
    name = "com_github_spacemonkeygo_spacelog",
    importpath = "github.com/spacemonkeygo/spacelog",
    sum = "h1:RC6RW7j+1+HkWaX/Yh71Ee5ZHaHYt7ZP4sQgUrm6cDU=",
    version = "v0.0.0-20180420211403-2296661a0572",
)

go_repository(
    name = "com_github_spaolacci_murmur3",
    importpath = "github.com/spaolacci/murmur3",
    sum = "h1:7c1g84S4BPRrfL5Xrdp6fOJ206sU9y293DDHaoy0bLI=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_spf13_afero",
    importpath = "github.com/spf13/afero",
    sum = "h1:m8/z1t7/fwjysjQRYbP0RD+bUIF/8tJwPdEZsI83ACI=",
    version = "v1.1.2",
)

go_repository(
    name = "com_github_spf13_cast",
    importpath = "github.com/spf13/cast",
    sum = "h1:oget//CVOEoFewqQxwr0Ej5yjygnqGkvggSE/gB35Q8=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_spf13_cobra",
    importpath = "github.com/spf13/cobra",
    sum = "h1:f0B+LkLX6DtmRH1isoNA9VTtNUK9K8xYd28JNNfOv/s=",
    version = "v0.0.5",
)

go_repository(
    name = "com_github_spf13_jwalterweatherman",
    importpath = "github.com/spf13/jwalterweatherman",
    sum = "h1:XHEdyB+EcvlqZamSM4ZOMGlc93t6AcsBEu9Gc1vn7yk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_spf13_pflag",
    importpath = "github.com/spf13/pflag",
    sum = "h1:zPAT6CGy6wXeQ7NtTnaTerfKOsV6V6F8agHXFiazDkg=",
    version = "v1.0.3",
)

go_repository(
    name = "com_github_spf13_viper",
    importpath = "github.com/spf13/viper",
    sum = "h1:VUFqw5KcqRf7i70GOzW7N+Q7+gxVBkSSqiXB12+JQ4M=",
    version = "v1.3.2",
)

go_repository(
    name = "com_github_src_d_envconfig",
    importpath = "github.com/src-d/envconfig",
    sum = "h1:/AJi6DtjFhZKNx3OB2qMsq7y4yT5//AeSZIe7rk+PX8=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_stretchr_objx",
    importpath = "github.com/stretchr/objx",
    sum = "h1:2vfRuCMp5sSVIDSqO8oNnWJq7mPa6KVP3iPIwFBuy8A=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_stretchr_testify",
    importpath = "github.com/stretchr/testify",
    sum = "h1:nOGnQDM7FYENwehXlg/kFVnos3rEvtKTjRvOWSzb6H4=",
    version = "v1.5.1",
)

go_repository(
    name = "com_github_syndtr_goleveldb",
    importpath = "github.com/syndtr/goleveldb",
    sum = "h1:fBdIW9lB4Iz0n9khmH8w27SJ3QEJ7+IgjPEwGSZiFdE=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_ugorji_go_codec",
    importpath = "github.com/ugorji/go/codec",
    sum = "h1:3SVOIvH7Ae1KRYyQWRjXWJEA9sS/c/pjvH++55Gr648=",
    version = "v0.0.0-20181204163529-d75b2dcb6bc8",
)

go_repository(
    name = "com_github_whyrusleeping_go_keyspace",
    importpath = "github.com/whyrusleeping/go-keyspace",
    sum = "h1:EKhdznlJHPMoKr0XTrX+IlJs1LH3lyx2nfr1dOlZ79k=",
    version = "v0.0.0-20160322163242-5b898ac5add1",
)

go_repository(
    name = "com_github_whyrusleeping_go_logging",
    importpath = "github.com/whyrusleeping/go-logging",
    sum = "h1:fwpzlmT0kRC/Fmd0MdmGgJG/CXIZ6gFq46FQZjprUcc=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_whyrusleeping_mafmt",
    importpath = "github.com/whyrusleeping/mafmt",
    sum = "h1:TCghSl5kkwEE0j+sU/gudyhVMRlpBin8fMBBHg59EbA=",
    version = "v1.2.8",
)

go_repository(
    name = "com_github_whyrusleeping_mdns",
    importpath = "github.com/whyrusleeping/mdns",
    sum = "h1:Y1/FEOpaCpD21WxrmfeIYCFPuVPRCY2XZTWzTNHGw30=",
    version = "v0.0.0-20190826153040-b9b60ed33aa9",
)

go_repository(
    name = "com_github_whyrusleeping_multiaddr_filter",
    importpath = "github.com/whyrusleeping/multiaddr-filter",
    sum = "h1:E9S12nwJwEOXe2d6gT6qxdvqMnNq+VnSsKPgm2ZZNds=",
    version = "v0.0.0-20160516205228-e903e4adabd7",
)

go_repository(
    name = "com_github_x_cray_logrus_prefixed_formatter",
    importpath = "github.com/x-cray/logrus-prefixed-formatter",
    sum = "h1:00txxvfBM9muc0jiLIEAkAcIMJzfthRT6usrui8uGmg=",
    version = "v0.5.2",
)

go_repository(
    name = "com_github_xordataexchange_crypt",
    importpath = "github.com/xordataexchange/crypt",
    sum = "h1:ESFSdwYZvkeru3RtdrYueztKhOBCSAAzS4Gf+k0tEow=",
    version = "v0.0.3-0.20170626215501-b2862e3d0a77",
)

go_repository(
    name = "com_google_cloud_go",
    importpath = "cloud.google.com/go",
    sum = "h1:e0WKqKTd5BnrG8aKH3J3h+QvEIQtSUcf2n5UZ5ZgLtQ=",
    version = "v0.26.0",
)

go_repository(
    name = "in_gopkg_check_v1",
    importpath = "gopkg.in/check.v1",
    sum = "h1:YR8cESwS4TdDjEe65xsg0ogRM/Nc3DYOhEAlW+xobZo=",
    version = "v1.0.0-20190902080502-41f04d3bba15",
)

go_repository(
    name = "in_gopkg_fsnotify_v1",
    importpath = "gopkg.in/fsnotify.v1",
    sum = "h1:xOHLXZwVvI9hhs+cLKq5+I5onOuwQLhQwiu63xxlHs4=",
    version = "v1.4.7",
)

go_repository(
    name = "in_gopkg_src_d_go_cli_v0",
    importpath = "gopkg.in/src-d/go-cli.v0",
    sum = "h1:mXa4inJUuWOoA4uEROxtJ3VMELMlVkIxIfcR0HBekAM=",
    version = "v0.0.0-20181105080154-d492247bbc0d",
)

go_repository(
    name = "in_gopkg_src_d_go_log_v1",
    importpath = "gopkg.in/src-d/go-log.v1",
    sum = "h1:heWvX7J6qbGWbeFS/aRmiy1eYaT+QMV6wNvHDyMjQV4=",
    version = "v1.0.1",
)

go_repository(
    name = "in_gopkg_tomb_v1",
    importpath = "gopkg.in/tomb.v1",
    sum = "h1:uRGJdciOHaEIrze2W8Q3AKkepLTh2hOroT7a+7czfdQ=",
    version = "v1.0.0-20141024135613-dd632973f1e7",
)

go_repository(
    name = "in_gopkg_urfave_cli_v1",
    importpath = "gopkg.in/urfave/cli.v1",
    sum = "h1:NdAVW6RYxDif9DhDHaAortIu956m2c0v+09AZBPTbE0=",
    version = "v1.20.0",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    importpath = "gopkg.in/yaml.v2",
    sum = "h1:obN1ZagJSUGI0Ek/LBmuj4SNLPfIny3KsKFopxRdj10=",
    version = "v2.2.8",
)

go_repository(
    name = "io_etcd_go_bbolt",
    importpath = "go.etcd.io/bbolt",
    sum = "h1:MUGmc65QhB3pIlaQ5bB4LwqSj6GIonVJXpZiaKNyaKk=",
    version = "v1.3.3",
)

go_repository(
    name = "io_opencensus_go",
    importpath = "go.opencensus.io",
    sum = "h1:8sGtKOrtQqkN1bp2AtX+misvLIlOmsEsNd+9NIcPEm8=",
    version = "v0.22.3",
)

go_repository(
    name = "org_golang_google_appengine",
    importpath = "google.golang.org/appengine",
    sum = "h1:/wp5JvzpHIxhs/dumFmF7BXTf3Z+dd4uXta4kVyO508=",
    version = "v1.4.0",
)

go_repository(
    name = "org_golang_google_genproto",
    importpath = "google.golang.org/genproto",
    sum = "h1:i1Ppqkc3WQXikh8bXiwHqAN5Rv3/qDCcRk0/Otx73BY=",
    version = "v0.0.0-20190425155659-357c62f0e4bb",
)

go_repository(
    name = "org_golang_google_grpc",
    importpath = "google.golang.org/grpc",
    sum = "h1:Hz2g2wirWK7H0qIIhGIqRGTuMwTE8HEKFnDZZ7lm9NU=",
    version = "v1.20.1",
)

go_repository(
    name = "org_golang_x_crypto",
    importpath = "golang.org/x/crypto",
    sum = "h1:1ZiEyfaQIg3Qh0EoqpwAakHVhecoE5wlSg5GjnafJGw=",
    version = "v0.0.0-20200221231518-2aa609cf4a9d",
)

go_repository(
    name = "org_golang_x_exp",
    importpath = "golang.org/x/exp",
    sum = "h1:c2HOrn5iMezYjSlGPncknSEr/8x5LELb/ilJbXi9DEA=",
    version = "v0.0.0-20190121172915-509febef88a4",
)

go_repository(
    name = "org_golang_x_lint",
    importpath = "golang.org/x/lint",
    sum = "h1:5hukYrvBGR8/eNkX5mdUezrA6JiaEZDtJb9Ei+1LlBs=",
    version = "v0.0.0-20190930215403-16217165b5de",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:w0SItUiQ4sBiXBAwWNkyu8Fu2Qpn/dtDIcoPkPDqjRw=",
    version = "v0.0.0-20200319234117-63522dbf7eec",
)

go_repository(
    name = "org_golang_x_oauth2",
    importpath = "golang.org/x/oauth2",
    sum = "h1:vEDujvNQGv4jgYKudGeI/+DAX4Jffq6hpD55MmoEvKs=",
    version = "v0.0.0-20180821212333-d2e6202438be",
)

go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
    sum = "h1:vcxGaoTs7kV8m5Np9uUNQin4BrLOthgV7252N8V+FwY=",
    version = "v0.0.0-20190911185100-cd5d95a43a6e",
)

go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
    sum = "h1:/WDfKMnPU+m5M4xB+6x4kaepxRw6jWvR5iDRdvjHgy8=",
    version = "v0.0.0-20200223170610-d5e6a3e2c0ae",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:tW2bmiBqwgJj/UpqtC8EpXEZVYOwU0yG4iWbprSVAcs=",
    version = "v0.3.2",
)

go_repository(
    name = "org_golang_x_tools",
    importpath = "golang.org/x/tools",
    sum = "h1:hKsoRgsbwY1NafxrwTs+k64bikrLBkAgPir1TNCj3Zs=",
    version = "v0.0.0-20191029190741-b9c20aec41a5",
)

go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
    sum = "h1:/atklqdjdhuosWIl6AIbOeHJjicWYPqR9bpxqxYG2pA=",
    version = "v0.0.0-20191011141410-1b5146add898",
)

go_repository(
    name = "org_uber_go_atomic",
    importpath = "go.uber.org/atomic",
    sum = "h1:OI5t8sDa1Or+q8AeE+yKeB/SDYioSHAgcVljj9JIETY=",
    version = "v1.5.0",
)

go_repository(
    name = "org_uber_go_multierr",
    importpath = "go.uber.org/multierr",
    sum = "h1:sFPn2GLc3poCkfrpIXGhBD2X0CMIo4Q/zSULXrj/+uc=",
    version = "v1.3.0",
)

go_repository(
    name = "org_uber_go_zap",
    importpath = "go.uber.org/zap",
    sum = "h1:nR6NoDBgAf67s68NhaXbsojM+2gxp3S1hWkHDl27pVU=",
    version = "v1.13.0",
)

go_repository(
    name = "com_github_cespare_xxhash",
    importpath = "github.com/cespare/xxhash",
    sum = "h1:a6HrQnmkObjyL+Gs60czilIUGqrzKutQD6XZog3p+ko=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_chzyer_logex",
    importpath = "github.com/chzyer/logex",
    sum = "h1:Swpa1K6QvQznwJRcfTfQJmTE72DqScAa40E+fbHEXEE=",
    version = "v1.1.10",
)

go_repository(
    name = "com_github_chzyer_readline",
    importpath = "github.com/chzyer/readline",
    sum = "h1:fY5BOSpyZCqRo5OhCuC+XN+r/bBCmeuuJtjz+bCNIf8=",
    version = "v0.0.0-20180603132655-2972be24d48e",
)

go_repository(
    name = "com_github_chzyer_test",
    importpath = "github.com/chzyer/test",
    sum = "h1:q763qf9huN11kDQavWsoZXJNW3xEE4JJyHa5Q25/sd8=",
    version = "v0.0.0-20180213035817-a1ea475d72b1",
)

go_repository(
    name = "com_github_datadog_zstd",
    importpath = "github.com/DataDog/zstd",
    sum = "h1:3oxKN3wbHibqx897utPC2LTQU4J+IHWWJO+glkAkpFM=",
    version = "v1.4.1",
)

go_repository(
    name = "com_github_dgraph_io_badger_v2",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/dgraph-io/badger/v2",
    sum = "h1:uBAA5oM9Gz9TrP01v9LxBGztE5rhtGeBxpF1IvxGGtw=",
    version = "v2.0.2",
)

go_repository(
    name = "com_github_dgraph_io_ristretto",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/dgraph-io/ristretto",
    sum = "h1:MQLRM35Pp0yAyBYksjbj1nZI/w6eyRY/mWoM1sFf4kU=",
    version = "v0.0.2-0.20200115201040-8f368f2f2ab3",
)

go_repository(
    name = "com_github_oneofone_xxhash",
    importpath = "github.com/OneOfOne/xxhash",
    sum = "h1:KMrpdQIwFcEqXDklaen+P1axHaj9BSKzvpUUfnHldSE=",
    version = "v1.2.2",
)

go_repository(
    name = "com_github_yuin_gopher_lua",
    importpath = "github.com/yuin/gopher-lua",
    sum = "h1:ZkM6LRnq40pR1Ox0hTHlnpkcOTuFIDQpZ1IN8rKKhX0=",
    version = "v0.0.0-20191220021717-ab39c6098bdb",
)

go_repository(
    name = "com_github_libp2p_go_libp2p_pubsub",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/libp2p/go-libp2p-pubsub",
    sum = "h1:ypZaukCFrtD8cNeeb9nnWG4MD2Y1T0p22aQ+f7FKJig=",
    version = "v0.2.6",
)

go_repository(
    name = "com_github_whyrusleeping_timecache",
    importpath = "github.com/whyrusleeping/timecache",
    sum = "h1:lYbXeSvJi5zk5GLKVuid9TVjS9a0OmLIDKTfoZBL6Ow=",
    version = "v0.0.0-20160911033111-cfcb2f1abfee",
)

go_repository(
    name = "com_github_akavel_rsrc",
    importpath = "github.com/akavel/rsrc",
    sum = "h1:zjWn7ukO9Kc5Q62DOJCcxGpXC18RawVtYAGdz2aLlfw=",
    version = "v0.8.0",
)

go_repository(
    name = "com_github_alecthomas_template",
    importpath = "github.com/alecthomas/template",
    sum = "h1:cAKDfWh5VpdgMhJosfJnn5/FoN2SRZ4p7fJNX58YPaU=",
    version = "v0.0.0-20160405071501-a0175ee3bccc",
)

go_repository(
    name = "com_github_alecthomas_units",
    importpath = "github.com/alecthomas/units",
    sum = "h1:qet1QNfXsQxTZqLG4oE62mJzwPIB8+Tee4RNCL9ulrY=",
    version = "v0.0.0-20151022065526-2efee857e7cf",
)

go_repository(
    name = "com_github_beorn7_perks",
    importpath = "github.com/beorn7/perks",
    sum = "h1:HWo1m869IqiPhD389kmkxeTalrjNbbJTC8LXupb+sl0=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_coreos_bbolt",
    importpath = "github.com/coreos/bbolt",
    sum = "h1:wZwiHHUieZCquLkDL0B8UhzreNWsPHooDAG3q34zk0s=",
    version = "v1.3.2",
)

go_repository(
    name = "com_github_coreos_go_systemd",
    importpath = "github.com/coreos/go-systemd",
    sum = "h1:Wf6HqHfScWJN9/ZjdUKyjop4mf3Qdd+1TvvltAvM3m8=",
    version = "v0.0.0-20190321100706-95778dfbb74e",
)

go_repository(
    name = "com_github_coreos_pkg",
    importpath = "github.com/coreos/pkg",
    sum = "h1:lBNOc5arjvs8E5mO2tbpBpLoyyu8B6e44T7hJy6potg=",
    version = "v0.0.0-20180928190104-399ea9e2e55f",
)

go_repository(
    name = "com_github_cpuguy83_go_md2man_v2",
    importpath = "github.com/cpuguy83/go-md2man/v2",
    sum = "h1:EoUDS0afbrsXAZ9YQ9jdu/mZ2sXgT1/2yyNng4PGlyM=",
    version = "v2.0.0",
)

go_repository(
    name = "com_github_daaku_go_zipexe",
    importpath = "github.com/daaku/go.zipexe",
    sum = "h1:wV4zMsDOI2SZ2m7Tdz1Ps96Zrx+TzaK15VbUaGozw0M=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_dgrijalva_jwt_go",
    importpath = "github.com/dgrijalva/jwt-go",
    sum = "h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=",
    version = "v3.2.0+incompatible",
)

go_repository(
    name = "com_github_dgryski_go_sip13",
    importpath = "github.com/dgryski/go-sip13",
    sum = "h1:RMLoZVzv4GliuWafOuPuQDKSm1SJph7uCRnnS61JAn4=",
    version = "v0.0.0-20181026042036-e10d5fee7954",
)

go_repository(
    name = "com_github_geertjohan_go_incremental",
    importpath = "github.com/GeertJohan/go.incremental",
    sum = "h1:7AH+pY1XUgQE4Y1HcXYaMqAI0m9yrFqo/jt0CW30vsg=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_geertjohan_go_rice",
    importpath = "github.com/GeertJohan/go.rice",
    sum = "h1:KkI6O9uMaQU3VEKaj01ulavtF7o1fWT7+pk/4voiMLQ=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_ghodss_yaml",
    importpath = "github.com/ghodss/yaml",
    sum = "h1:wQHKEahhL6wmXdzwWG11gIVCkOv05bNOh+Rxn0yngAk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_go_flutter_desktop_hover",
    importpath = "github.com/go-flutter-desktop/hover",
    sum = "h1:Ca2xO/3zNsmkfTBuSnaH2HzPG5tpwWl8KMQkXm0/2n0=",
    version = "v0.39.0",
)

go_repository(
    name = "com_github_go_kit_kit",
    importpath = "github.com/go-kit/kit",
    sum = "h1:Wz+5lgoB0kkuqLEc6NVmwRknTKP6dTGbSqvhZtBI/j0=",
    version = "v0.8.0",
)

go_repository(
    name = "com_github_go_logfmt_logfmt",
    importpath = "github.com/go-logfmt/logfmt",
    sum = "h1:MP4Eh7ZCb31lleYCFuwm0oe4/YGak+5l1vA2NOE80nA=",
    version = "v0.4.0",
)

go_repository(
    name = "com_github_go_stack_stack",
    importpath = "github.com/go-stack/stack",
    sum = "h1:5SgMzNM5HxrEjV0ww2lTmX6E2Izsfxas4+YHWRs3Lsk=",
    version = "v1.8.0",
)

go_repository(
    name = "com_github_google_btree",
    importpath = "github.com/google/btree",
    sum = "h1:0udJVsspx3VBr5FwtLhQQtuAsVc79tTq0ocGIPAU6qo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_google_go_github",
    importpath = "github.com/google/go-github",
    sum = "h1:N0LgJ1j65A7kfXrZnUDaYCs/Sf4rEjNlfyDHW9dolSY=",
    version = "v17.0.0+incompatible",
)

go_repository(
    name = "com_github_google_go_querystring",
    importpath = "github.com/google/go-querystring",
    sum = "h1:Xkwi/a1rcvNg1PPYe5vI8GbeBY/jrVuDX5ASuANWTrk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_grpc_ecosystem_go_grpc_middleware",
    importpath = "github.com/grpc-ecosystem/go-grpc-middleware",
    sum = "h1:Iju5GlWwrvL6UBg4zJJt3btmonfrMlCDdsejg4CZE7c=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_grpc_ecosystem_go_grpc_prometheus",
    importpath = "github.com/grpc-ecosystem/go-grpc-prometheus",
    sum = "h1:Ovs26xHkKqVztRpIrF/92BcuyuQ/YW4NSIpoGtfXNho=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_grpc_ecosystem_grpc_gateway",
    importpath = "github.com/grpc-ecosystem/grpc-gateway",
    sum = "h1:bM6ZAFZmc/wPFaRDi0d5L7hGEZEx/2u+Tmr2evNHDiI=",
    version = "v1.9.0",
)

go_repository(
    name = "com_github_hashicorp_go_version",
    importpath = "github.com/hashicorp/go-version",
    sum = "h1:3vNe/fWF5CBgRIguda1meWhsZHy3m8gCJ5wx+dIzX/E=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_jonboulle_clockwork",
    importpath = "github.com/jonboulle/clockwork",
    sum = "h1:VKV+ZcuP6l3yW9doeqz6ziZGgcynBVQO+obU0+0hcPo=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_julienschmidt_httprouter",
    importpath = "github.com/julienschmidt/httprouter",
    sum = "h1:TDTW5Yz1mjftljbcKqRcrYhd4XeOoI98t+9HbQbYf7g=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_kr_logfmt",
    importpath = "github.com/kr/logfmt",
    sum = "h1:T+h1c/A9Gawja4Y9mFVWj2vyii2bbUNDw3kt9VxK2EY=",
    version = "v0.0.0-20140226030751-b84e30acd515",
)

go_repository(
    name = "com_github_logrusorgru_aurora",
    importpath = "github.com/logrusorgru/aurora",
    sum = "h1:bqDmpDG49ZRnB5PcgP0RXtQvnMSgIF14M7CBd2shtXs=",
    version = "v0.0.0-20200102142835-e9ef32dff381",
)

go_repository(
    name = "com_github_matttproud_golang_protobuf_extensions",
    importpath = "github.com/matttproud/golang_protobuf_extensions",
    sum = "h1:4hp9jkHxhMHkqkrB3Ix0jegS5sx/RkqARlsWZ6pIwiU=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_mwitkow_go_conntrack",
    importpath = "github.com/mwitkow/go-conntrack",
    sum = "h1:F9x/1yl3T2AeKLr2AMdilSD8+f9bvMnNN8VS5iDtovc=",
    version = "v0.0.0-20161129095857-cc309e4a2223",
)

go_repository(
    name = "com_github_nkovacs_streamquote",
    importpath = "github.com/nkovacs/streamquote",
    sum = "h1:E2B8qYyeSgv5MXpmzZXRNp8IAQ4vjxIjhpAf5hv/tAg=",
    version = "v0.0.0-20170412213628-49af9bddb229",
)

go_repository(
    name = "com_github_oklog_ulid",
    importpath = "github.com/oklog/ulid",
    sum = "h1:EGfNDEx6MqHz8B3uNV6QAib1UR2Lm97sHi3ocA6ESJ4=",
    version = "v1.3.1",
)

go_repository(
    name = "com_github_otiai10_copy",
    importpath = "github.com/otiai10/copy",
    sum = "h1:PH7IFlRQ6Fv9vYmuXbDRLdgTHoP1w483kPNUP2bskpo=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_otiai10_curr",
    importpath = "github.com/otiai10/curr",
    sum = "h1:TJIWdbX0B+kpNagQrjgq8bCMrbhiuX73M2XwgtDMoOI=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_otiai10_mint",
    importpath = "github.com/otiai10/mint",
    sum = "h1:BCmzIS3n71sGfHB5NMNDB3lHYPz8fWSkCAErHed//qc=",
    version = "v1.3.1",
)

go_repository(
    name = "com_github_prometheus_client_golang",
    importpath = "github.com/prometheus/client_golang",
    sum = "h1:9iH4JKXLzFbOAdtqv/a+j8aewx2Y8lAjAydhbaScPF8=",
    version = "v0.9.3",
)

go_repository(
    name = "com_github_prometheus_client_model",
    importpath = "github.com/prometheus/client_model",
    sum = "h1:S/YWwWx/RA8rT8tKFRuGUZhuA90OyIBpPCXkcbwU8DE=",
    version = "v0.0.0-20190129233127-fd36f4220a90",
)

go_repository(
    name = "com_github_prometheus_common",
    importpath = "github.com/prometheus/common",
    sum = "h1:7etb9YClo3a6HjLzfl6rIQaU+FDfi0VSX39io3aQ+DM=",
    version = "v0.4.0",
)

go_repository(
    name = "com_github_prometheus_procfs",
    importpath = "github.com/prometheus/procfs",
    sum = "h1:sofwID9zm4tzrgykg80hfFph1mryUeLRsUfoocVVmRY=",
    version = "v0.0.0-20190507164030-5867b95ac084",
)

go_repository(
    name = "com_github_prometheus_tsdb",
    importpath = "github.com/prometheus/tsdb",
    sum = "h1:YZcsG11NqnK4czYLrWd9mpEuAJIHVQLwdrleYfszMAA=",
    version = "v0.7.1",
)

go_repository(
    name = "com_github_rogpeppe_fastuuid",
    importpath = "github.com/rogpeppe/fastuuid",
    sum = "h1:gu+uRPtBe88sKxUCEXRoeCvVG90TJmwhiqRpvdhQFng=",
    version = "v0.0.0-20150106093220-6724a57986af",
)

go_repository(
    name = "com_github_russross_blackfriday_v2",
    importpath = "github.com/russross/blackfriday/v2",
    sum = "h1:lPqVAte+HuHNfhJ/0LC98ESWRz8afy9tM/0RK8m9o+Q=",
    version = "v2.0.1",
)

go_repository(
    name = "com_github_shurcool_sanitized_anchor_name",
    importpath = "github.com/shurcooL/sanitized_anchor_name",
    sum = "h1:PdmoCO6wvbs+7yrJyMORt4/BmY5IYyJwS/kOiWx8mHo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_soheilhy_cmux",
    importpath = "github.com/soheilhy/cmux",
    sum = "h1:0HKaf1o97UwFjHH9o5XsHUOF+tqmdA7KEzXLpiyaw0E=",
    version = "v0.1.4",
)

go_repository(
    name = "com_github_tcnksm_go_latest",
    importpath = "github.com/tcnksm/go-latest",
    sum = "h1:IWllFTiDjjLIf2oeKxpIUmtiDV5sn71VgeQgg6vcE7k=",
    version = "v0.0.0-20170313132115-e3007ae9052e",
)

go_repository(
    name = "com_github_tmc_grpc_websocket_proxy",
    importpath = "github.com/tmc/grpc-websocket-proxy",
    sum = "h1:LnC5Kc/wtumK+WB441p7ynQJzVuNRJiqddSIE3IlSEQ=",
    version = "v0.0.0-20190109142713-0ad062ec5ee5",
)

go_repository(
    name = "com_github_ugorji_go",
    importpath = "github.com/ugorji/go",
    sum = "h1:j4s+tAvLfL3bZyefP2SEWmhBzmuIlH/eqNuPdFPgngw=",
    version = "v1.1.4",
)

go_repository(
    name = "com_github_valyala_bytebufferpool",
    importpath = "github.com/valyala/bytebufferpool",
    sum = "h1:GqA5TC/0021Y/b9FG4Oi9Mr3q7XYx6KllzawFIhcdPw=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_valyala_fasttemplate",
    importpath = "github.com/valyala/fasttemplate",
    sum = "h1:tY9CJiPnMXf1ERmG2EyK7gNUd+c6RKGD0IfU8WdUSz8=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_xiang90_probing",
    importpath = "github.com/xiang90/probing",
    sum = "h1:eY9dn8+vbi4tKz5Qo6v2eYzo7kUS51QINcR5jNpbZS8=",
    version = "v0.0.0-20190116061207-43a291ad63a2",
)

go_repository(
    name = "in_gopkg_alecthomas_kingpin_v2",
    importpath = "gopkg.in/alecthomas/kingpin.v2",
    sum = "h1:jMFz6MfLP0/4fUyZle81rXUoxOBFi19VUFKVDOQfozc=",
    version = "v2.2.6",
)

go_repository(
    name = "in_gopkg_resty_v1",
    importpath = "gopkg.in/resty.v1",
    sum = "h1:CuXP0Pjfw9rOuY6EP+UvtNvt5DSqHpIxILZKT/quCZI=",
    version = "v1.12.0",
)

go_repository(
    name = "org_golang_x_time",
    importpath = "golang.org/x/time",
    sum = "h1:SvFZT6jyqRaOeXpc5h/JSfZenJ2O330aBsf7JfSUXmQ=",
    version = "v0.0.0-20190308202827-9d24e82272b4",
)

go_repository(
    name = "com_github_google_gofuzz",
    importpath = "github.com/google/gofuzz",
    sum = "h1:A8PeW59pxE9IoFRqBp37U+mSNaQoZ46F1f0f863XSXw=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_google_renameio",
    importpath = "github.com/google/renameio",
    sum = "h1:GOZbcHa3HfsPKPlmyPyN2KEohoMXOhdMbHrvbpl2QaA=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_json_iterator_go",
    importpath = "github.com/json-iterator/go",
    sum = "h1:9yzud/Ht36ygwatGx56VwCZtlI/2AD15T1X2sjSuGns=",
    version = "v1.1.9",
)

go_repository(
    name = "com_github_maoxs2_go_jsonrpc2",
    importpath = "github.com/maoxs2/go-jsonrpc2",
    sum = "h1:iO8GM9xWv3iRzA1sBVhyhEJAbjGR9gJo/2K8TYufPfY=",
    version = "v0.0.0-20200326130745-a6a35812420f",
)

go_repository(
    name = "com_github_modern_go_concurrent",
    importpath = "github.com/modern-go/concurrent",
    sum = "h1:ZqeYNhU3OHLH3mGKHDcjJRFFRrJa6eAM5H+CtDdOsPc=",
    version = "v0.0.0-20180228061459-e0a39a4cb421",
)

go_repository(
    name = "com_github_modern_go_reflect2",
    importpath = "github.com/modern-go/reflect2",
    sum = "h1:Esafd1046DLDQ0W1YjYsBW+p8U2u7vzgW2SQVmlNazg=",
    version = "v0.0.0-20180701023420-4b7aa43c6742",
)

go_repository(
    name = "com_github_rogpeppe_go_internal",
    importpath = "github.com/rogpeppe/go-internal",
    sum = "h1:RR9dF3JtopPvtkroDZuVD7qquD0bnHlKSqaQhgwt8yk=",
    version = "v1.3.0",
)

go_repository(
    name = "in_gopkg_errgo_v2",
    importpath = "gopkg.in/errgo.v2",
    sum = "h1:0vLT13EuvQ0hNvakwLuFZ/jYrLp5F3kcWHXdRggjCE8=",
    version = "v2.1.0",
)

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:JgcxKXxCjrA2tyDP/aNU9K0Ck5Czfk6C7e2tMw7+bSI=",
    version = "v0.0.0-20190513183733-4bf6d317e70e",
)

go_repository(
    name = "org_uber_go_tools",
    importpath = "go.uber.org/tools",
    sum = "h1:0mgffUl7nfd+FpvXMVz4IDEaUSmT1ysygQC7qYo7sG4=",
    version = "v0.0.0-20190618225709-2cfd321de3ee",
)
