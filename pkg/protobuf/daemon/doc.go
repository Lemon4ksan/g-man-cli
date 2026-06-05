// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package daemon contains auto-generated Protocol Buffer and gRPC code definitions
for interacting with the G-MAN daemon.

The package provides interfaces and structures to call and stream daemon methods
like GetStatus, PlayGame, ExecAction, and StreamEvents.

# Key Components

  - [DaemonServiceClient]: gRPC client helper to communicate with the local/remote running daemon process.
  - [DaemonServiceServer]: Server interface used to implement the daemon control handler.
  - [GetStatusRequest]: Request payload to query connection status and system memory metrics.
  - [ExecActionRequest]: Request structure containing target game actions and key-value parameter arguments.
*/
package daemon
