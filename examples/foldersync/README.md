# Folder-sync app
This sample-app allows peers to join a _shared folder_, where each user will 
own a folder named as their username. The local peer can drop new files to its 
folder, where a filewatcher will automatically detect new files and sync to 
other joined peers.


## Design
The main components of the applications are:
- A `DB` using new Threads V2.
- IPFS Lite to download files.

The `DB` will hold all files metadata, which is a JSON document derived from 
the following structure:
```
type userFolder struct {
	ID    core.InstanceID
	Owner string
	Files []file
}

type file struct {
	ID               string
	FileRelativePath string
	CID              string

	IsDirectory bool
	Files       []file
}
```
In summary, each folder is represented by a `userFolder` which is owned by a 
user. This folder has a list of `file` which has file metadata such as:
- A unique uuid
- A relative path from the root of the shared folder: `userXXX/file.txt`
- The CID of the file, which can be fetched *out-of-band* with IPFS Lite.

The original intention is to allow arbitrary nested folders and files, but 
currently it has only one level support for files. It's on the plans to enable 
folder discovery to allow nested trees.

Currently, when a new file is discovered by a peer, it will try to fetch it 
using an `ipfslite` client, which will leveraged connected peers. By default, 
the `ipfslite` peer doesn't boostrap a DHT, but it can be enabled in a single 
line. Future versions will make this optional through flags. In the same vein, 
we can provide other sources from which fetch files such as the Filecoin 
network.

## Usage
When a peer wants to bootstrap a new shared folder, to invite others, it has to 
run: `go run main.go util.go client.go -name bob`. This will create a new 
`DB` with an underlying new `Thread`, and will print an invitation link that 
other peers can use to join the shared folder.

Joining peers will run: 
`go run main.go util.go client.go -name alice -inviteLink xxxxxxx` to join 
the shared folder. The invite link has format: 
`<thread-addr>?<follow-key>&<read-key>`.

Current flags are:
- `name`: to setup the peer name, which will be his sharedfolder name
- `repo`: path to the repo that will hold data of `DB`
- `sharedFolder`: path to where will be the shared folder containing all peer 
folders with their files
- `inviteLink`: if provided is used to join an existing _sharedFolder_

Flags have reasonable defaults.

## Tests
This app *now* has a pretty heavy test setup, where tunning a parameter 
simulates:
- A number N of peers joining a shared folder
- How many random files will each peer generate
- How big each random file will be
- How many core peers will be used to join the _sharedFolder_. This means, 
X peers will be used as invitation links by others in a round-robin fashion 
possibly to avoid DoSing all joining the same peer. May also be used to 
test how this impact syncing perf.
- How many seconds provide to the peers to sync before asserting convergence.

After the sync timeout triggers, the test will assert if all `DB` of peers 
are the same, and if all the files were fetched (that to say, each peer shared 
folder contains all peers folders and their files).

Take into consideration that tests configuration should be tuned to avoid 
timeouts and high overload, since all peers are running on the same host.
