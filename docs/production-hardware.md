# Available production computer

The user supplied a photograph of Windows System About on 4 October 2026. Record only the resource information needed for deployment; the photograph, device identifiers and Windows product identifier are not repository artifacts.

| Resource | Visible information |
|---|---|
| CPU | Intel Core i5-8300H, displayed base frequency 2.30 GHz |
| RAM | 8 GB, displayed speed 2667 MHz |
| System | 64-bit Windows, x64 processor; precise edition/build not shown |
| Storage | Approximately 238 GB usable SSD (256 GB class), approximately 466 GB usable HDD (500 GB class) |

The existing React/TypeScript frontend builds into static HTML, CSS and JavaScript. The Go application serves those files, performs the API operations and uses SQLite. Node and a browser are development/build/QA dependencies, not server runtime requirements. Preserve this stack while measuring the actual workload. Rewriting in Angular or Rust does not establish a resource or usability improvement.

Use the SSD for the live database, WAL and application working files. An internal HDD can hold an additional local copy but does not supply off-host disaster recovery. No OS replacement or modification of any existing accounting installation is authorised by a hardware photograph.

The plan's dedicated Linux deployment remains an option. A Windows x64 package can be built without a separate container/virtual-machine layer; compile evidence alone does not prove Windows startup, private-file access, power behaviour or backup recovery. Establish the intended host operation before selecting the production service setup.

Still required: exact OS/build and support status, machine availability during society operations, SSD health/free space, power and sleep/reboot settings, connectivity/ingress, measured concurrent workload, encrypted off-host backups and custody. The original plan's concurrent-use acceptance starts with 20 active sessions and includes manual entries, receipt generation and realistic reads. Synthetic measurements on the development Mac do not establish performance on this Windows computer.
