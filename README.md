# webchat
It is a salable chat service based on socket.io and PUBSUB.

There are three repositories related to this project.

chat server: https://github.com/kyopark2014/webchat-golang-chat

profile server: https://github.com/kyopark2014/webchat-golang-profile

web client: https://github.com/kyopark2014/webchat-js-webclient



### Basic Call Flow

Client <-> Server : Socket.io
 
#### Overall
![alternative text](http://www.plantuml.com/plantuml/png/ZP51Rzf048Nlyoj6N0AAKj4SUuYo1fIuQGB2H56Fote2AnOltHsdfLN_UyVOZN6Ar711lFVDlfd7TMGTkDmwY1jRx7JGMpHEnuY_0EMJQBDP1LyM-SWx7_1fRPaEZWh1Vw_5n0bLupxa5DwgYZdOPScK6bzazYVX47x3MPiOUUy8er2ckuje5CZpVaUZawkSHDFtevFte-f_gBC5LI3wm6iFSCQM7M4lBInRpTONFLZRdF3PUrxw7m3Qi0-ecky4m7Zdo5Hen2TQgjskgwNt7z_yleHSirxgI3YM1xwLL1BE9-CqGvNXV0n7_jS7TJy5K7XnSTKyZv_mTh6OuSraWNtdLxOO83IMGED40BCciy4NxMnkUG-Y7OzFx-1rG7GbqkLmEBr3-2UfZfezIbH6ZBFWdwqZL8CsKJOMi797Md9Rezc-ClKBjRDrobNLZ7Z65mKPndxqPaFymVe1YDC-_9XaetcQJAh1mns70mDf9hbJS_1-2z2zyEPuL-nNHmoWk_BvUSsrQmNYCXJjtF50KdU4KbB_SpPz660JAN-dMBrjNFKw9u0OdGuYyLzigSsct3LCSwTqd7n9irjKDGKWwcj9ZTJqeaucNbEHbrlt1m00)

#### Online
![alternative text](http://www.plantuml.com/plantuml/png/ZPF1RXen48RlynGZBcaEI8LZ3rNMGDMLcZPYXQeUZNSIh1WFjMTBQTLthxUBHHojL3Y0_Czd_zySk8kYWtHxXqOxz-Zu2F7P-eCEUe_6wHZnDs3wA6sUdm9tlbsmuu1VTrRetwGAWOz3g5oY1fn3I-5rMea4k-s4QiE-iRy8ty8VkBaqHZavIj_1cbuANaue4ClfG8jHbL6qlXvTNOzM_qETzTI3o45s370ZLXpXHEbe3NvmVCJPL7WwmpSF6zLirEtj17Qs9NnabYt_1D16E61r_bK0XfqZ8vPznFf-zkey1_ZUKKUuNYth1gi6rqk0LelUwaYu_FQvkly2KE7qVNyRtw5Xxya8zASKhfSfIoRDObshLIe6DR3pWfsFiAfuTxAFJnCKHfMV7ihJA0e76W0eUXyswbFTV4ILXodDvJawTwjBebumBJdxWy9f08lMD2AXivVsRTQuNCbtIfcgGBBHh6pBQrJZSIbHbggyJY3TZkvauy7faPNo1WKmwf-1KZ_7XNsHZijVY3loRV_t1FWB)

#### Offline

![alternative text](http://www.plantuml.com/plantuml/png/ZPB1Rjim38RlV0esbww704kFFHIraWmpi6v3Z6BOKP6PHAWYTXBTBXlsxgDd0JQA0eik0icF_7yAkijYatJ7WCw6i0ly0ldHnoURx15Ti3dZRq0znhh7VU8kjaiEdF3RmGlzgrGfySkGD46f0UVKKdgThKIIttP2jUFO-5-4x-4FN8wAMKw1C2lbUbCmhL0YbjCJBT-syXPQdu-kpqUh_w71H-f1v2G71hWKBu5mPcps3Z_e8-DYBZnVuBkl3wPvC5TNCpZubd37B5l-2M2TwE2gt0R0SGZan7FCMDylNiNN03ywwWWtwrNTODNWPWNGMh5RcmbNtpzNzry0AfpVzjruWuvZr7xGI9EAQqwTDCjDRTOg36PWh_7YOgId4sR7l9-XC9eofCXeAGid6W2OAFIJTE5FTVCHJIaN8QDTlDlrBpU_vTZV4miJ0ngEKZUOIFyAFTCWsUAHShPxxHs0SK1HA-rcp9W9qrBmpvHE1Pokf2MIiXC2m1t5jl-X07y1)

Once one of client in a chatroom is not online, PUBSUB is not a good way to commnicate messages. So, the application server saves the message in Queue too using linked list. Then, once a client is back online, the message can be delivered to the client robustly.

### PUBSUB ISSUES

For one-to-one chat, published messages from Sender will return since the sender also do SUBSCRIBE into PUBSUB server.
It may be used for acknowlegement for the sent message. But it also requires special management for UI because an user usually prefers to use legacy message experience where it shows if a message is failed to send by some reason, it should be displayed in the chat dialog so that the user recognizes and then tries again if needs.
So, I think it is a redundacy for PUBSUB when it is applied for one-to-one chat.

![image](https://user-images.githubusercontent.com/52392004/82962776-2801c100-9ffc-11ea-91bc-ebb94843d553.png)

where Sender and Receiver are subscribed to a chat room with chat-id.

In mobile environment, the state of user client can move to online to offline which means that Sender may not sucess to send a message based on the state of Receiver. So, the architecture should support store and forward approach.

The proposed architecture is using PUBSUB to endpoints which doesn't have the redundancy and some of benefits to notice the received message and manages a single connection for multiple chat rooms.

![image](https://user-images.githubusercontent.com/52392004/82962567-89756000-9ffb-11ea-8e5a-7eb737fd6f37.png)

where Sender will publish to Receiver directly and messages can be delivered consistently without the state of Receiver.

### OVERALL STRUCTURE

There are two connection ways between client and server. Socket.io is used for chat session and REST API is used for call logs.

![image](https://user-images.githubusercontent.com/52392004/82965685-c6455500-a003-11ea-91ed-974b845d856d.png)

where REDIS is for PUBSUB and DYNAMO DB is for storage for call logs. Notice that the diagram shows the simple flow for messaging where many flows are ignored to explain the basic flows.
