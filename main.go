package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
)

func main() {

	var (
		streamIn             io.Reader
		streamOut, streamErr io.Writer
	)

	kubelet := flag.Bool("kubelet", false, "Use kubelet URI")
	https := flag.Bool("https", false, "Use HTTPS")
	host := flag.String("host", "", "kubelet/apiserver host/ip address with port")
	namespace := flag.String("namespace", "", "pod's namespace")
	pod := flag.String("pod", "", "pod's name")
	container := flag.String("container", "", "containers's name")
	cmd := flag.String("cmd", "ls -la /", "command to execute")
	flag.Parse()

	if *host == "" || *namespace == "" || *pod == "" || *container == "" {
		fmt.Fprint(os.Stderr, "usage: ./spdy --kubelet --host k8s-node-1:10250 --namespace kube-system --pod node-exporter-iuwg7 --container node-exporter --cmd \"ls -la\"\n")
		fmt.Fprint(os.Stderr, "usage: ./spdy --host apiserver:443 --namespace kube-system --pod node-exporter-iuwg7 --container node-exporter --cmd \"ls -la\"\n")
		os.Exit(1)
	}

	var URL, proto string
	if *https {
		proto = "https"
	} else {
		proto = "http"
	}

	if *kubelet {
		// kubelet exec
		// /exec/kube-system/node-exporter-iuwg7/node-exporter?command=ls&command=-la&error=1&output=1
		URL = fmt.Sprintf("%s://%s/exec/%s/%s/%s", proto, *host, *namespace, *pod, *container)
	} else {
		// apiserver exec
		//http://127.0.0.1:8080/api/v1/namespaces/default/pods/nginx-ingress-controller-pqeuk/exec?command=pwd&container=nginx-ingress-lb&container=nginx-ingress-lb
		URL = fmt.Sprintf("%s://%s/api/v1/namespaces/%s/pods/%s/exec", proto, *host, *namespace, *pod)
	}
	localOut := &bytes.Buffer{}
	localErr := &bytes.Buffer{}

	url, _ := url.ParseRequestURI(URL)
	config := restclient.ContentConfig{
		GroupVersion:         &unversioned.GroupVersion{Group: "x"},
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
	}
	c, err := restclient.NewRESTClient(url, "", config, -1, -1, nil, nil)
	if err != nil {
		fmt.Printf("failed to create a client: %v", err)
		return
	}
	req := c.Post()

	for _, param := range strings.Fields(*cmd) {
		req.Param("command", param)
	}
	if *kubelet == false {
		req.Param("container", *container)
	}

	req.Param(api.ExecStdoutParam, "1")
	streamOut = localOut

	req.Param(api.ExecStderrParam, "1")
	streamErr = localErr

	conf := &restclient.Config{
		Host:     URL,
		Insecure: true,
	}
	e, err := NewExecutor(conf, "POST", req.URL())
	if err != nil {
		fmt.Printf("unexpected error: %v", err)
		return
	}

	err = e.Stream([]string{}, streamIn, streamOut, streamErr, false)
	fmt.Printf("stdout:\n%s\n", streamOut)
	fmt.Printf("stderr:\n%s\n", streamErr)

	fmt.Printf("err:\n%v\n", err)
}
