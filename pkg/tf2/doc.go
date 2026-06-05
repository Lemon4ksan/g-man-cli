// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package tf2 implements a Team Fortress 2 concrete game adapter.

It integrates official G-MAN Steam modules, mapping their specific representations and operations (such as metal
smelting, layout sorting, and trade offer checks) into unified interfaces used by the core daemon.

# Key Components

  - [Driver]: The concrete adapter wrapping TF2 GC connection, backpack cache, crafting manager, and trade client.
  - [GetSectionPriority]: Resolves the presentation section of a TF2 item for sorting and cataloging.
*/
package tf2
