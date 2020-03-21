bazel run //:gazelle -- -go_prefix github.com/ngchain/ngcore
bazel run //:gazelle -- update-repos -from_file=go.mod