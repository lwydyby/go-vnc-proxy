<template>
  <a-card>
    <div>
      <div class="operator">
        <a-button @click="connect" type="primary" :disabled="hasConnected">连接</a-button>
        <a-button @click="disConnect" type="danger" style="margin-left: 8px" :disabled="!hasConnected">断开连接</a-button>
        <a-dropdown placement="bottomCenter" style="margin-left: 8px" :disabled="!hasConnected">
          <a-button>工具栏</a-button>
          <a-menu slot="overlay">
            <a-menu-item>
              <a target="_blank" @click="copy">剪切板</a>
            </a-menu-item>
            <a-menu-item>
              <a target="_blank" @click="opensSession(1)">CTRL_ALT_F1</a>
            </a-menu-item>
            <a-menu-item>
              <a target="_blank" @click="opensSession(2)">CTRL_ALT_F2</a>
            </a-menu-item>
            <a-menu-item>
              <a target="_blank" @click="opensSession(3)">CTRL_ALT_F3</a>
            </a-menu-item>
            <a-menu-item>
              <a target="_blank" @click="opensSession(4)">CTRL_ALT_F4</a>
            </a-menu-item>
            <a-menu-item>
              <a target="_blank" @click="opensSession(5)">CTRL_ALT_F5</a>
            </a-menu-item>
            <a-menu-item>
              <a target="_blank" @click="openDel()">CTRL_ALT_DEL</a>
            </a-menu-item>
          </a-menu>
        </a-dropdown>
      </div>
      <a-modal
        title="剪切板"
        :visible="copyVisible"
        @ok="handleCopyOk"
        @cancel="handleCopyCancel"
      >
        <a-textarea placeholder="请填写内容" v-model="copyValue" :rows="5"/>
      </a-modal>
      <a-modal
        title="连接配置"
        :visible="visible"
        @ok="handleOk"
        @cancel="handleCancel"
      >
        <a-form-model
          ref="ruleForm"
          :model="vnc"
          :label-col="labelCol"
          :wrapper-col="wrapperCol"
          width="60%"
        >
          <a-form-model-item label="虚机Id" prop="instanceId">
            <a-input v-model="vnc.instanceId" disabled/>
          </a-form-model-item>
        </a-form-model>
      </a-modal>

      <div id="screen" style="width: 100%;height: 600px;margin-top: 5px">
      </div>
    </div>
  </a-card>
</template>

<script>

import RFB from '@novnc/novnc/core/rfb';
import KeyTable from "@novnc/novnc/core/input/keysym";


export default {
  name: "vnc",
  data() {
    return {
      rfb: null,
      hasConnected:false,
      IsClean: false,
      connectNum: 0, //重连次数
      vnc: {
        instanceId: undefined,
      },
      visible: false,
      labelCol: {span: 4},
      wrapperCol: {span: 14},
      copyVisible: false,
      copyValue: "",
      token:"",
      VNC:"localhost:8080"  //vnc proxy地址
    }

  },
  created() {
      this.token=this.getUrlKey("token")
      this.instanceId=this.getUrlKey("instance_id")
  },
  methods: {
    getUrlKey: function (name) {
      return decodeURIComponent((new RegExp('[?|&]' + name + '=' + '([^&;]+?)(&|#|;|$)').exec(location.href) || [, ""])[1].replace(/\+/g, '%20')) || null
    },
    // vnc连接断开的回调函数
    disconnectedFromServer(msg) {
      this.$message.info("已断开连接")

    },
    errorPassword(status,reason){
      this.$message.error("密码错误,请重新输入");
      this.visible = true
      this.vnc.password = null
      this.hasConnected=false
    },
    // 连接成功的回调函数
    connectedToServer() {
      console.log('success')
    },
    //连接vnc的函数
    connectVnc() {
      if (this.rfb) {
        this.rfb.disconnect()
      }
      const url = "ws://" + this.VNC + "/ws?token="+this.token+"&instanceId=" + this.vnc.instanceId
      this.rfb = new RFB(document.getElementById('screen'), url, {
        // 向vnc 传递的一些参数，比如说虚拟机的开机密码等
        // credentials: {password: this.vnc.password}
      });
      this.rfb.addEventListener('connect', this.connectedToServer);
      this.rfb.addEventListener('disconnect', this.disconnectedFromServer);
      this.rfb.addEventListener('securityfailure',this.errorPassword)
      this.rfb.scaleViewport = true;  //scaleViewport指示是否应在本地扩展远程会话以使其适合其容器。禁用时，如果远程会话小于其容器，则它将居中，或者根据clipViewport它是否更大来处理。默认情况下禁用。
      this.rfb.resizeSession = true; //是一个boolean指示是否每当容器改变尺寸应被发送到调整远程会话的请求。默认情况下禁用
      this.rfb.showDotCursor = true;
      this.hasConnected=true
    },
    handleCancel() {
      this.visible = false
    },
    handleOk() {
      this.connectVnc()
      this.visible = false
    },
    connect() {
      this.visible = true
      this.vnc.password=null
    },
    opensSession(num) {
      let type;
      switch (num) {
        case 1:
          type = KeyTable.XK_F1
          break
        case 2:
          type = KeyTable.XK_F2
          break
        case 3:
          type = KeyTable.XK_F3
          break
        case 4:
          type = KeyTable.XK_F4
          break
        default:
          type = KeyTable.XK_F5
          break
      }
      this.rfb.sendKey(KeyTable.XK_Control_L, "ControlLeft", true);
      this.rfb.sendKey(KeyTable.XK_Alt_L, "AltLeft", true);
      this.rfb.sendKey(type, "F" + num, true);
      this.rfb.sendKey(type, "F" + num, false);
      this.rfb.sendKey(KeyTable.XK_Alt_L, "AltLeft", false);
      this.rfb.sendKey(KeyTable.XK_Control_L, "ControlLeft", false);
    },
    openDel(){
      this.rfb.sendKey(KeyTable.XK_Control_L, "ControlLeft", true);
      this.rfb.sendKey(KeyTable.XK_Alt_L, "AltLeft", true);
      this.rfb.sendKey(KeyTable.XK_Delete, "Del", true);
      this.rfb.sendKey(KeyTable.XK_Control_L, "ControlLeft", false);
      this.rfb.sendKey(KeyTable.XK_Alt_L, "AltLeft", false);
      this.rfb.sendKey(KeyTable.XK_Delete, "Del", false);
    },
    copy() {
      this.copyValue = ""
      this.copyVisible = true
    },
    handleCopyOk() {
      const str = String(this.copyValue);
      // fixme this.rfb.clipboardPasteFrom(str); does not work
      setTimeout(() => {
        for (let i = 0, len = str.length; i < len; i++) {
          this.rfb.sendKey(str.charCodeAt(i));
        }
      }, 100);
      this.copyVisible = false
    },
    handleCopyCancel() {
      this.copyVisible = false
    },
    getPwd(){
      getVncPassword(this.vnc.instanceId).then(res=>{
        this.vnc.password=res.data.data.password
      })
    },
    disConnect(){
      this.rfb.disconnect()
      this.rfb=null
      this.hasConnected=false
    }
  },

}
</script>

<style scoped>

</style>
