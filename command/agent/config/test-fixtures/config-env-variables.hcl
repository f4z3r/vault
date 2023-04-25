# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

pid_file = "./pidfile"

auto_auth {
  method "aws" {
    mount_path = "$VAULT_AWS_MOUNTPATH"
    namespace = "$VAULT_NAMESPACE"
    config = {
      type = "iam"
      role = "$VAULT_AWS_ROLE"
      region = "eu-west-1"
      test = {
        inner = "$VAULT_INNER"
      }
    }
  }

  sink {
    type = "file"

    config = {
      path = "$VAULT_SINK_PATH"
      test = ["item1", "$VAULT_ITEM"]
    }

    aad     = "foobar"
    dh_type = "curve25519"
    dh_path = "/tmp/file-foo-dhpath"
  }
}
