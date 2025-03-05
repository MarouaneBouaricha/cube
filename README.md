# Cube
Orchestrator from scratch

## Architecture

## How To
```shell
A Cli to interact with cube orchestrator.

Cube is an orchestrator based on containers.

Usage:
  cube [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  manager     Manager command to operate a Cube manager node.
  node        Node command to list nodes.
  run         Run a new task.
  status      Status command to list tasks.
  stop        Stop a running task.
  worker      Worker command to operate a Cube worker node.

Flags:
  -h, --help   help for cube
```

## Manager
```shell
cube manager command.

The manager controls the orchestration system and is responsible for:
- Accepting tasks from users
- Scheduling tasks onto worker nodes
- Rescheduling tasks in the event of a node failure
- Periodically polling workers to get task updates

Usage:
  cube manager [flags]

Flags:
  -d, --dbType string      Type of datastore to use for events and tasks ("memory" or "persistent") (default "memory")
  -h, --help               help for manager
  -H, --host string        Hostname or IP address (default "0.0.0.0")
  -p, --port int           Port on which to listen (default 5555)
  -s, --scheduler string   Name of scheduler to use. (default "epvm")
  -w, --workers strings    List of workers on which the manager will schedule tasks. (default [localhost:5556])
```

## Worker
```shell
cube worker command.

The worker runs tasks and responds to the manager's requests about task state.

Usage:
  cube worker [flags]

Flags:
  -d, --dbtype string   Type of datastore to use for tasks ("memory" or "persistent") (default "memory")
  -h, --help            help for worker
  -H, --host string     Hostname or IP address (default "0.0.0.0")
  -n, --name string     Name of the worker (default "worker-b6d6dd7d-e236-4c6a-b390-8c63a6245590")
  -p, --port int        Port on which to listen (default 5556)
```

## Run Tasks
```shell
cube run command.

The run command starts a new task.

Usage:
  cube run [flags]

Flags:
  -f, --filename string   Task specification file (default "task.json")
  -h, --help              help for run
  -m, --manager string    Manager to talk to (default "localhost:5555")
```

### List Nodes
```shell
cube node command.

The node command allows a user to get the information about the nodes in the cluster.

Usage:
  cube node [flags]

Flags:
  -h, --help             help for node
  -m, --manager string   Manager to talk to (default "localhost:5555")
```
```shell
NAME               MEMORY (MiB)     DISK (GiB)     ROLE       TASKS     
localhost:5556     15684            467            worker     0         
localhost:5557     15684            467            worker     0
```
### List running tasks
```shell
cube status command.

The status command allows a user to get the status of tasks from the Cube manager.

Usage:
  cube status [flags]

Flags:
  -h, --help             help for status
  -m, --manager string   Manager to talk to (default "localhost:5555")
```