`Note: I decided to try and turn this into a full-fledge game. Because of that, I started putting the gameplay part of the project in a private repository and stopped adding to this repository. I do still want to open source as much of my code as I can (without giving away the entire project), so I have several other open source repos that I moved a lot of code to:`
1. `github.com/unitoftime/glitch` - Rendering
2. `github.com/unitoftime/flow` - General game components features
3. `github.com/unitoftime/ecs` - ECS framework
4. You can also go here to see the repositories I have in my profile: [https://github.com/unitoftime](https://github.com/unitoftime)

### Welcome!
If you are here, then you may have come from my tutorial series on YouTube. If not, you can go check it out:
* [YouTube Playlist](https://www.youtube.com/playlist?list=PL_r0j2F4Hkj8KZ6jNJPCW3aDH--aWrn-T)
* [YouTube Channel](https://www.youtube.com/channel/UCrcOrUcsMYRMqTfAy-IG0rg)

If you have any feedback let me know!

### Compiling and Running
Get the code
```
go get github.com/unitoftime/mmo
```

The current instructions are slightly complicated
```
cd cmd/
mkdir build

make all
# Everything should build - There will be one step where you generate a key, This is for the TLS connection between your client and proxy. You can leave all of the options blank (ie just hit enter until the key starts generating)

bash run.sh
# This will start the server, then the proxy, then launch a desktop client
```

If you want to test the wasm you'll have to host the `build/` folder at some url. I use a simple go webserver to host my folder. Also, when you access the hosted URL, the browser will complain that the key at `localhost:port` isn't a part of any Certificate Authority. This is because you just manually generated the key. You have to skip the security check. Chrome had a way for me to allow arbitrary keys for localhost connections, so I enabled that.

You'll have to manually start the server and proxy binaries too:
```
# Shell 1
cd cmd/build/ && ./server
# Shell 2
cd cmd/build/ && ./proxy
# Shell 3
# Whatever webserver command you use to serve it
```

### Licensing
1. Code: MIT License.
2. Artwork: All rights reserved.
