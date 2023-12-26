# Beehive

Beehive is a distributed task scheduling framework built upon the small world network model. This repository contains its core code, implemented in the Go programming language. It is designed to showcase the architecture and scheduling process of Beehive. It is not a specific real-world application version.

## Architecture

![Beehive Architecture](https://github.com/Trump-last/Beehive/blob/46623531056551ed9d905e47c4b5e4ddf4ee9abd/Screenshots/architecture.png)

Beehive is primarily composed of three parts.

1. **Node Scheduler**: Each node in the cluster runs a Node Scheduler, which is responsible for the local scheduling of task requests, task global routing, and maintaining network connections and resource states with neighboring nodes. This decentralized scheduling design enables low-latency task execution and efficient resource allocation.
2. **Task Request Dispatcher**: This component swiftly distributes incoming tasks across the node schedulers using a round-robin method. This approach ensures an even distribution of tasks, preventing bottlenecks and enabling a distribution load of 10K tasks per second. The dispatcher’s design allows for horizontal scaling during periods of high demand.
3. **Cluster Meta State Service **(CMS): The CMS module continuously monitors the health status and network connectivity of each node in the cluster, providing critical information for the overall system’s stability and efficiency. It also manages the network connection information for nodes that join or leave the system.

They are located in the Scheduler4 folder, the LB folder, and the GCS folder, respectively.



## Scheduling Process

![Scheduling Process](https://github.com/Trump-last/Beehive/blob/46623531056551ed9d905e47c4b5e4ddf4ee9abd/Screenshots/taskflower.jpg)

Beehive's architecture integrates node schedulers, essential for efficiently distributing tasks among nodes according to available resources and task requirements. This process employs a novel task management approach that entails selecting one of three possible actions—Local Commit, Task Routing, and Neighbor Probing—in each scheduling round.

**Action 1: Local Submit.** When the local node has sufficient resources to accommodate a task, the Node Scheduler opts for a Local Submit. As depicted in Fig.1, the task is directly submitted and committed to execution at the local node, minimizing latency and maximizing task performance by promptly utilizing available resources.

**Action 2: Neighbor Probing.** When local node resources are constrained, inspired by the power of two, the Node Scheduler can employ a strategy known as Neighbor Probing. As shown in Fig.2, In this approach, the Node Scheduler queries neighboring nodes to determine if there are sufficient resources to execute the task, and the selection of an appropriate node is based on the responses received

**Action 3: Task Routing.** If the local node and its neighboring nodes are overloaded, the Node Scheduler selects Task Routing, as shown in Fig.3 direct transfer. A task routing request is sent to a neighboring node based on the small-world network connections. The chosen route node can be the least loaded node or a randomly selected node among neighboring nodes. Task routing ensures global resource utilization. 

## Test Procedure

1. Prepare an ETCD cluster to store metadata for cluster nodes.
2. Deploy the GCS program, with the main code located in GCS/main.go; modify the corresponding entry parameters.
3. Deploy the node program, with the main code in Scheduler4/NodeServer/main.go; modify the corresponding entry parameters.
4. Deploy the LB program, with the main code in LB/main.go; modify the corresponding entry parameters.
5. Use gRPC to send tasks to LB. Currently, the temporary test file is in LB/client/main.go, available for users to set up testing.
