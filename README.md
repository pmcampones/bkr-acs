# BKR Agreement on Common Subset

Implementation of the Agreement on Common Subset (ACS) communication primitive following the key ideas from Ben-Or, Kelmer, and Rabin ([BKR](https://dl.acm.org/doi/10.1145/197917.198088))

## Agreement on Common Subset

The Agreement on Common Subset primitive (aka. [Agreement on Core Set](https://dl.acm.org/doi/10.1145/167088.167109), aka. [Asynchronous Interactive Consistency](https://dl.acm.org/doi/abs/10.1007/s00446-002-0083-3), aka. [Asynchronous Common Subset](https://dl.acm.org/doi/10.1145/3372297.3423364), aka. [Vector Consensus](https://ieeexplore.ieee.org/document/1524949)) is a primitive that allows a set of nodes to agree on a set of values formed from inputs specific to each node.
The length of the agreed set is at least the number of correct nodes in the system.

This primitive is employed in asynchronous networks and is a common step to achieving atomic/total order broadcast under this network model.

### BKR

The main idea of BKR is to reduce the ACS problem to several parallel instances of Byzantine Reliable Broadcast ([BRB](https://www.sciencedirect.com/science/article/pii/089054018790054X)) and Asynchronous Binary Agreement ([ABA](https://www.sciencedirect.com/science/article/pii/089054018790054X)).
Each node in the system will propose a value using BRB and whether this value belongs in the output set is determined by having all nodes participate in a ABA.

This key idea is used in recent asynchronous Byzantine Fault Tolerant (BFT) State Machine Replication (SMR) systems such as [HoneyBadgerBFT](https://dl.acm.org/doi/10.1145/2976749.2978399), [BEAT](https://dl.acm.org/doi/10.1145/3243734.3243812), and [PACE](https://dl.acm.org/doi/10.1145/3548606.3559348).

### Asynchronous Binary Agreement (ABA)

In the [ABA](https://www.sciencedirect.com/science/article/pii/089054018790054X) primitive, each node proposes a binary value and the output is a binary value that is proposed by at least one correct node.
The implementation of ABA follows the algorithm from Mostéfaoui, Moumen, and Raynal ([MMR](https://dl.acm.org/doi/10.1145/2785953)).

Instead of faithfully following the aforementioned algorithm, we reduce ABA to Binding Crusader Agreement ([BCA](https://dl.acm.org/doi/10.1145/3519270.3538426)) and a coin toss.
Additionally we use an optimization presented in the BCA paper to reduce the number of communication rounds by reusing information from different ABA rounds (External Validity property).

### Coin Tossing

Efficient ABA algorithms, such as MMR, require a distributed coin tossing primitive. We follow the algorithm of [Cachin, Kursawe, and Shoup](https://dl.acm.org/doi/10.1145/343477.343531) to implement this primitive.
In particular, our implementation uses a *2t*-unpredictable strong coin.

The elliptic curves used in the coin tossing algorithm are [Ristretto255](https://ristretto.group/).
The [CIRCL](https://github.com/cloudflare/circl) library was used compute the group operations and the [discrete log equivalence proofs](https://link.springer.com/chapter/10.1007/3-540-48071-4_7).

During the setup phase, we assume the leader is honest. 
A more secure implementation would require a [distributed key generation protocol for high threshold values](https://www.usenix.org/conference/usenixsecurity23/presentation/das).

### Usage

To try the code, you must run several instances, each of which will be a node in the network.
At least one of the nodes must be the contact node, which will be the entry point for the other nodes to join the network.

Messages only start being exchanged after all nodes have joined the network.
The number of nodes in the network, as well as the address of the contact are set in the configuration file **config.properties**.

Additionally, each node must know its own address.
This information is not in the configuration file and must instead be passed as an argument when running the node.
