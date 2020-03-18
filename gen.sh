bazel run //:gazelle -- -go_prefix github.com/ngin-network/ngcore
bazel run //:gazelle -- update-repos -from_file=go.mod