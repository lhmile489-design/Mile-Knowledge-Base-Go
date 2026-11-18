import("/src/mygo.ts").then(async m=>{await m.SpikeService.winOp("toggleMaximize");return m.SpikeService.winState()})
