# webchat
It is a salable chat service based on socket.io and PUBSUB.

There are three repositories related to this project.

chat server: https://github.com/kyopark2014/webchat-golang-chat

profile server: https://github.com/kyopark2014/webchat-golang-profile

web client: https://github.com/kyopark2014/webchat-js-webclient


### PROPERTIES
#### Single channel per  a client
The channel is reusuable between client and server. It is important to reduce the connection complexity of the server so that the number of capability to cover the number of user in a node.

#### Single event queue
The messages and notification are using a single queue so that the order of messsage and notification can be aligned easily. Also, it has a benefit to manage messages and notifications compared with other messagers which use diffenre queue for message and notification.

#### Scalable architecture 
It provides horizontal scaling.

#### Profile functionanity
User profile can be share between members which can improve user experience.

#### Delivery and display notification
The notifications of delivery and display are useful for convinient chat service. Think about the popular messenger, slack which is also using websocket(socket.io) but it doesn't provide deliver and notifcation functionalities.

#### Groupchat (#of members is not limited)
The groupchat is based on PUBSUB which has a strength for groupchat. Also, the number of participants is not limited and can be easily scaled up.

#### Chat client in Web Browser
Web browser has a benefit for availability since all computer has a browser which measn that users can easily message service.


### STRUCTURE

There are two connection ways between client and server. Socket.io is used for chat session and REST API is used for call logs.

![image](https://user-images.githubusercontent.com/52392004/82965685-c6455500-a003-11ea-91ed-974b845d856d.png)

where REDIS is for PUBSUB and DYNAMO DB is for storage for call logs. Notice that the diagram shows the simple flow for messaging where many flows are ignored to explain the basic flows.


### Basic Call Flow

Client <-> Server : Socket.io
 
#### Login and Profile Update
![alternative text](http://www.plantuml.com/plantuml/png/ZPDFJzjC4CRlyob6kP28brUKOmy8Eqd1XJPH3EAyh8TubSrEkXv3QTNltjcO912GYY_MFlERF_DezwmH4wLTUhJ6-tlFZz0iN5YPQ9PelMaQ_0seNsxiOXwv3TMGFKUyhPtGEfB5o8-Tc7kbEfnZHN5VpKIYksk52ikXTBy8Z-4F7EmwDlBa2HkbR1eANaOe4ilJYeRlZ_8UMdmS7NySpVw5UXSeWSXHQWOu42UUi5S49yw8uz23sbM4zynonpy1Z1Mjapt_Sm3BtfDDQ8FJsUHBSJK6qEhYh5kP85Xy-xHtFe7AYBap3U58A_negIMStkJbJOvPYRCHl0OoF3ewtHJ7ptXnVJt5y_4rzZtFNHWWR6AAZN8bR4rsWG_6kyh94wZsgi3a4dOf6ibKEpa-dbmYlErMak0qyhtpf60_DH-yR3WRFKD3estVZ5AQ1zhcT-LFaoTfIWw1h62_ORiW-T_n03IoAwgM36T5FiOC8B0GyWCbqszh2hX-3oxCqUYwHlx3TbKPPQGcN7NzW4A5AH5WxS3QcaY9IYcHUGcmvytvU-lkxNjus84RBeT8qiQ0M-5LHjxPY-fl_siv-Jx0hckw2XhgBPjvJoKzRLVNst1Lb1UOTL40LRU-TTB6gdod09pfnkav0lm5)

#### Online
![alternative text](http://www.plantuml.com/plantuml/png/ZPF1Qjj048RlynI3Bsa8WJh77DhikYaLfD16X9xNgqcyPBtZxewQkgNltX4hOrKOcfEPJ_zyyrkWco8kIx-Dw5sCZv5Ve3o7j7FPRT57Lmh-1aH4w_ppK-O-TIkEdF7R9WWDJz3ap2yLsgYf6kVSKPvJ8vB3kXTgFASs_29y2t_Wx7Yno3uI5atveHHCdr0cblsELgUhd8eshu_UlZvg_XUD8T4GHCwoOO0p2H895zQLuF6ZlcHSNWfVBl7DbmVRFjZpymLiGaVuo2nh_WdWlAZOZByMGCqIVDYv9DXyNixcAu3EYLkxGjX-lGFutbDFMEreMhp_063myjsmYzVeEINo0ZeDK5SKHlPE0YSq8zSjnQrALlUDlLKEjZgkfWwTgcEYi1NF5Sh_wR7ObgS52YkVTBcukCQce4LEmwjyZv19-fp66QRrXlzurxIVq5OPWD93nmCQkzWZiPDCHp7yeBnNN9Cpipb69lhbR7NiPEP6UxYSoNKdpuUoYqwpfuyV0xFR5SzEJ-WDfMxuXW3-0W00)

#### Offline

![alternative text](http://www.plantuml.com/plantuml/png/ZLHHRnez47pVNx7X9KGHKiZZ9-Lhpe6s9wLL2aLzDhubPyNOrDuBfLN_U_ToW0D2RNX1dfsT6O_kxZQnZjmi7HhjtCA5DQGdwrSwwYKQfrF2Nu3oKzeyFSRG-Eekk13nMssPNYP5Z67TWSe9gwE7M54yHWlcQESDKsc2dzgVX5Vm6ywpO-AD8qp2CcqeU3sXI84tAxex6UKajNmxTVnsQl4lghEUMYA6o7K0E6VBZh2dTB86tql9E1nm60on_p1JqvcwkEX1RIl2HGWy3py0j64HBhR_2a2KsHgxqfwn_3GyEby3L9hrN2V2aR2_DzGGTk9OJ74osc5VPkFPk8LQ6XGu-B-Lm__G1E_9S0UDb20k6CqsU8aX5855Hk2fgDvDIYKg0AeJkCQpiwo1_OMsZgeBW9iRL6WJXiMYRGNb323ylKcpak24N6i-O1vawotJOmyvj4K8ibs-l9HHcMGj-7Rb7KJY9lgzHEOACN68j5NgJnvcqu-ekgF4p3dzFcQ-Mou7v7YY27Lu9rEJUS8bfQGVAM7_Vb9y_Z36AnmCnMkKTJj_01-OC3nJR67uMbENja9w9iy9rzOvd1DMvAomP33VuCiR1kGhRDTX7q5btTTEszsDm0Uta8ValfGoYWEAZ7Bx6TyrMXmhgfrp95sTjBTfvVJVt3CZc-V5O-yiToiLjDyAW3y0)

Once one of client in a chatroom is not online, PUBSUB is not a good way to commnicate messages. So, the application server saves the message in Queue too using linked list. Then, once a client is back online, the message can be delivered to the client robustly.

The proposed architecture is using PUBSUB to endpoints which doesn't have the redundancy and some of benefits to notice the received message and manages a single connection for multiple chat rooms. Notice that see https://github.com/kyopark2014/webchat-golang-chat if you need more.
Sender will publish to Receiver directly and messages can be delivered consistently without the state of Receiver.

#### execeptional case based on application

![alternative text](http://www.plantuml.com/plantuml/png/ZPFHRzCm4CRVxwyuvMMRfagqZpp0x2w8I1qKGWMlhdDhhRbsiI-CWlZVSOXHkhJIzfBalllbk-ySvIQo2jpkB6fbxRtrZn0VZDkhe7QehOeHVmCYebJwOHDywvgvjpxWrwrX-jSH8VZ7NfGsKJtkGqDXh0hcODOjKwMzgyqlmbVm1ow6YP4FbZ0cIdUXuBX3WJmVzZG_7-KSMhqSBL-EYkTGQnnr8Fh0MmzmmOOjOI5LD1hVfKF6wOJzP8gNvRTvkVnIVVmW5ZWNTNbLmDOqXFVUyzh_152QVK2Hxn8WZM2ZpLuvnkfkEgfd08rYjLQHy5R2zvPQmkLArgl_HN57CQeDOPqCgK1HuwTLkIe114xUT9Ru6hLtZZH3gfAOd99clLPil4EHvUoRMkbQppzNiamzaBtDx8dDxDH6PlcSZNoIfjZ5JO7oACtrTPAhY1AzwqxxBGJYDZWyK8JCzMibRh5SrUzHPflBxkkr-oiusI8zBAg11EWxmpfPph7be1mp3LdpWy8XWqSvRil5oE9aj1ZU7ysVZhnpU36IRyWrtOy3y1S0)



### Groupchat

#### Create

As shown in the bellow picture, A will make a groupchat using a create event. Then IM server will save the participant list and subscribe the groupchat toward PUBSUB server. In this project, I will use a redis server for PUBSUB operation.
The last participants execept owner will have an event which notices that "SUBSCRIBE" is required to get groupchat messages. After all of subscribe request were completed, the members in the groupchat can share messages easily based on PUBSUB.

This picture shows what relationship of client, IM and REDIS.
![image](https://user-images.githubusercontent.com/52392004/84559883-ac34a200-ad79-11ea-8f8b-8562cf91cc5e.png)

The basic groupchat flow is shown as bellow.
![alternative text](http://www.plantuml.com/plantuml/png/fPH1Jzj048NlyokUt0D5GDN0BL8gOfEsaQY42AZdpNg8LooxONUTDAtwtpkE8pSGGf5wiV7iDxFllK4vZqc5L3zOQ6NjdVLBYlV6pLLG3z1MnOXV102vqlUpu6jN5jxwWE-LIRm-6ORWbsqnjqArk0ybX-VLOKh1JEl4O-tTnFnaVAJVTF3tnfXMbX65qeqesZxXm3wjvboyAEL5TFnsTFHsTFWlr1h73GWVKkM93f99bf7bAXgDpn8oUYV9d_Jm_kesdzpcXuSPLQPat7cVflu7aJoG5TQmIndUNPfPbN9RCocTVC1myyutxuBObLWtaaX8HfkvSWdZRxrdtwVFli-aerH9JLLaNEGWUgovPhIwY0O338-H7wF0aWD3zkPO9vRRLeXUttFwO3m48t8-CVo26trV99xv-AAF6JiEQjEWruReQal2fOcfZyYscrf73ci_pJoY8Ku-DKRGbq2kHyER4MQzJ6HBJOuQqTAl5enjPrR6TaXhHZ1vJehhSJv29ZEs5Gj9DnM35o9GrX2mh8oka7Kj6Kn4uCVQ12uRefsnDDO-DPbaZ5XORBdOLI9_N8udNv4ZY_Kqwc2c_7Iw30UrV9l0AVhxuJRkZ0JRX79qr8VrgjoD9N2gWzkjDxsTnlwMzKRBVWl5hgez3ivUSo1N2F4VuizU4pyOj1dfIYL4Zj5urovKbIj0Zf_yept4MRRNSBFt5jdTZrIx9UHTOJS5ANT-YwvmpgviVlQ8_W00)



#### RESTART

If one groupchat is not activate any more, it should be desubscribed in order to reduce resources.
So, after the groupchat was deactivated, it can be activate when one of participants wants to send a message.
But since there is no session based on PUBSUB machanism, it may not activate again.
This picture shows how it reactivate.

![alternative text](http://www.plantuml.com/plantuml/png/fPJDRjf04CVlynGZNz8eGYfmur35TbBLLJ-gegZdPJt0Aek7x8v3qwhltZ6ca6AieeO3nhC__pz65jTHJ91wvT4Qx-UUDn3lNRKsmQpGUXCZ_WJKJsRi_I9mNPKvUmxuRUc4jYTf2BnfXvbNgiKvb1IEfwb8SBDQgB1SJTqFmjVm2ouEZb6UF65KoZQXuFa91M9vMbD-Cif9j3WVlJqVJV-5UbTH0o87MJB0XJZHMabcehFuLfUCmu7mO8YlljnbqxliyZA1fIi9vymou-y0nWe7J7VNR7VD0InxJrOSLn6BZyEZ-z7H_HYWD69c9XBUP0ZmK5DDsBe2G8g3DuqCJZ1PnKM2PguriIGngl5e6Xq01GvSnQWjnLctDfLeroZ8SqmMmsHFgjNLrGGzcx83Go2fGtLAP_zyskG9j7qtKONnhwJlFnUVjdwuSRBiLsdfLYJGeH0-pwc0UQHkfMPb9N9ricow4zWjKaNqUvf_BRBRRR3EiVxuK4pVOOP9h6VH1ZSZ3FHGkw0kSmvjovPliuzmqWy7hjV5JUCpEdFVtRa-UyYeNeSAJOFU2datLK-3SN-3hOKIBmW__f_mYPwsRtfdr-tqprQo_QJJIAVxxFb-S4rLsVnZ0Vm6)

OFFLINE -> ONLINE

![alternative text](http://www.plantuml.com/plantuml/png/ZPD1RzD048NlyokclhHH5OcKMmzLlIaLHj0YXGfntAmdoIgR7RCx9WJ4VyUE5uSwaThJPD_xPkP58zz6qK7gZKEZdLiuta9SMr_fe3Ted8uHVmCYej9clGnS-tBAZWD-NLcXlPE7mDjELAwXEfn3IM6evYB1pckXmh2VsL-4R-0Fd1ysHjavmjXGfWq5_piKY6LNqVHqb5DeyNhqtUlH_2NKMKyjY1nanG3dOiKHPaf7Q_3UyHOdO-7n12y-FwdPangDCbZPad31B7F-2Q2DSC0yLPNg5C2mSsJ4iezOV9eCtbU3zrk0KekUwqXufn3WUqqrORSL078StxHZyEmChv4NYpOwWEhagqRBDd6PDRa2Pij91j2glMbMPDQOGo_VgKQDme6wniE6bcRloGXUf3tZ6_Pj7G4qG6g_lEpdxkUKkA4OzP8Y198w-9I85obGQitnUkZa_tJFMEaWrjXAUq5demoeBk27n-9Xh-9H8iUwF1wI8XqPpr8zF7ufxh-TOFfyTLLgePD4doqhoTaV57RzNqvNRBpcnyTYzXxLCohP_R5wxt15DP1loPVjPmVm5m00)


#### REFER

![alternative text](http://www.plantuml.com/plantuml/png/ZLHVRze-57s_d-8eB_rL5TAFbgT8T2K9rJ9jxLHQJNiqoQMnwiRKTc1isdTVZ1a4njO-MVVSS_-VvCewOLpph508fMPABya-oNekZ7X6eOIr-440a8ZYwT7efYvJhRJ1bqewNdj6nkXb01FLiW9TcvBD8JfopiXfupWlT3sHtnd_qqywsLMqRgKOjcKLlYdgUjYmTgivfqTREKhDtquTlvqwUesgPCsU26rSfObEd7IA4IN2oWBNxPBHxpdTw-E_pm_9v24vFOseaYLZfhMRwcz4ed3QOBHvaysRRjwCgAtaP27dedR8F_KFxFC3--B07X2Lmecfi8mi0T5BmmqZT4Cq7EA-OXILSB1ihTGrbi92ACEYN85j6wkahRZ4S4WqGk_ITu4OT-FhyPrli3LRr6S_EukHMsGGM0WbIpIMpJiox1fJ1wXBZz7CI-4Ohc8Odh4nNABJloMgjMFe1Plrx6DHL2YKP8UbL0gLM32Y3xVvJGHUSEs8wn8-PbidJ1hxzsC-UOyay95555nXedF4EqThUZJfOiblH9Gbih-cJu-dlzXFV-7Hj8kb0Tccl-dCFI1_evgNdTrq5o3gC3ncsY2Qj6NJkpmPHwyl8qDacwajZ9pw2xmqqb_0ftdixmymYFz9RGdT_DiU47MyMU85ehJOEpAKj6wZaJ_nRQnVritjVNxz5Ne6eTHQN3OYojR-65OiE0HvMVlFThwdQsaHH8tMlIVbwDa-HeSxyk21HWAurSUMj7_gW7REV9GssD46lSjiHxhYklH_HA9V)


#### DEPART 

![alternative text](http://www.plantuml.com/plantuml/png/ZPF1RXCn48RlynIZlPIgYiGsjnvGrziWLW9KDKISdTrfOjNn17jC28XtnzPQQh8KqLmc_kRB_3E7t0RHNkBEOg-jVRHyWF1at5vxlSFUwX3m5s3wADq_RJn7DxHisUFNhH7A7MoyvyC8bKtMgBCVo4zf8-BDEWfrFRkb-Kdu5dx3nNDYaACb3Cdgyr9msY5FBCSzjI-kygBQlLvTl5vj_gTQuoYBo5ws370XHYnXfNGmFRwtVC1w9ZohyStzIYrNwlAoWgqP21-PPSq_07GlQN1JgYgr1KW9Odgprqwm-rHFtjUJznnWqABNEX3UAGJu5YaIZga0ZeMGlrDEYY6LWx4M1yf3SATTrDOUWTo0sGHeSFOkf-8DtYtkcuSlM6tggl0cqTvoe0AJbt6AlRfABSjwmDFjh0a2dYHwzpSllovXddPfovHWtDX-dds3SPzEf7-EACScCNtvs2q_uAR6gjpOL32snwphP9sqCaXSdLDLsEcnPrFcqodppDjpsXPsDcMwyUfpgilseLEBOjoI6_8V0-0F)


### How to Run
This project shows the operation using docker. 

#### Dynamo db (local)
```c
$ docker run -d -p 8000:8000 amazon/dynamodb-local
```

#### Redis (docker)
```c
$ docker run -d -p 6379:6379 redis:latest
```

#### RUN in visual studio code
1) Profile Server
```c
$ git clone https://github.com/kyopark2014/webchat-golang-profile
$ go run main.go
```

2) Webchat server
```c
$ git clone https://github.com/kyopark2014/webchat
$ go run main.go
```

3) Run client
```c
$ git clone https://github.com/kyopark2014/webchat-js-webclient
$ code 
```
Note: I recommand "liver server" in visual studio code in order to run "index.html" in webchat-js-webclient


#### RUN in Docker
Note that the ip address in the server should be replaced from "localhost" to "your IP".

1) Profile Server
```c
$ git clone https://github.com/kyopark2014/webchat-golang-profile
$ go run main.go
```

2) Webchat server
```c
$ git clone https://github.com/kyopark2014/webchat
$ docker build -t webchat-golang-profile:v1 .
$ docker run -d -p 4040:4040 -e AWS_ACCESS_KEY="key" -e AWS_SECRET_ACCESS_KEY="key" webchat-golang-profile:v1
```

3) Run client
```c
$ git clone https://github.com/kyopark2014/webchat-js-webclient
$ docker build -t webclient:v1 .
$ docker run -d -p 8080:80 webclient:v1
```
- Open "http://ip-address:8080/ using chrome.

