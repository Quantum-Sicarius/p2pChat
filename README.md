# p2pChat
p2pChat is a program I wrote to learn Golang. It started off as a simple peer-to-peer tcp chat program where I simply could telnet into it.

Node:

![alt tag](https://raw.githubusercontent.com/Quantum-Sicarius/p2pChat/master/screenshots/Screenshot%20from%202015-11-30%2021-33-31.png)

Telnet:

![alt tag](https://raw.githubusercontent.com/Quantum-Sicarius/p2pChat/master/screenshots/Screenshot%20from%202015-11-30%2021-34-26.png)

Soon I was able to have two nodes talking to each other no more telnet! I added some fancy colors to make the messages more readable.

![alt tag](https://github.com/Quantum-Sicarius/p2pChat/blob/master/screenshots/Screenshot%20from%202015-12-02%2018-14-52.png)

I stated working with data synchronization. I calculate a checksum of all the keys in my dataset. I used this approach as it used the least bandwidth.

![alt tag](https://github.com/Quantum-Sicarius/p2pChat/blob/master/screenshots/Screenshot%20from%202015-12-04%2016-59-49.png)

Later on it morphed into a p2p chat sync program in which nodes communicate their current data state and updates other nodes to keep them all up to date with the latest data.

![alt tag](https://github.com/Quantum-Sicarius/p2pChat/blob/master/screenshots/Screenshot%20from%202015-12-04%2016-59-52.png)

Image with no all debugging off:

![alt tag](https://github.com/Quantum-Sicarius/p2pChat/blob/master/screenshots/Screenshot%20from%202015-12-04%2017-07-55.png)
