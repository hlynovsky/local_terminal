package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	readBufferSize  = 1024
	writeBufferSize = 1024
	serverPort      = ":8765"
	podPrefix       = "user-"
	maxNameLength   = 10
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	activePods sync.Map
)

type TerminalSession struct {
	wsConn      *websocket.Conn
	sizeChan    chan remotecommand.TerminalSize
	doneChan    chan struct{}
	k8sClient   *kubernetes.Clientset
	podName     string
	namespace   string
}

func generatePodName(userID string) string {
	timestamp := time.Now().UnixNano()
	uniqueStr := fmt.Sprintf("%s-%d", userID, timestamp)
	hash := sha256.Sum256([]byte(uniqueStr))
	hashStr := hex.EncodeToString(hash[:])
	if len(hashStr) > maxNameLength-len(podPrefix) {
		hashStr = hashStr[:maxNameLength-len(podPrefix)]
	}
	return podPrefix + hashStr
}

func getKubernetesConfig() (*rest.Config, error) {
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}
	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

func NewTerminalSession(wsConn *websocket.Conn, client *kubernetes.Clientset, podName string) *TerminalSession {
	return &TerminalSession{
		wsConn:    wsConn,
		sizeChan:  make(chan remotecommand.TerminalSize),
		doneChan:  make(chan struct{}),
		k8sClient: client,
		podName:   podName,
		namespace: "default",
	}
}

func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		return 0, err
	}
	return copy(p, message), nil
}

func (t *TerminalSession) Write(p []byte) (int, error) {
	err := t.wsConn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *TerminalSession) Close() error {
	close(t.doneChan)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deletePolicy := metav1.DeletePropagationBackground
	return t.k8sClient.CoreV1().Pods(t.namespace).Delete(ctx, t.podName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
}

func (t *TerminalSession) WaitForPodRunning(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pod, err := t.k8sClient.CoreV1().Pods(t.namespace).Get(ctx, t.podName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}
		}
	}
}

func createPod(ctx context.Context, client *kubernetes.Clientset, podName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"app": "terminal-pod",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "terminal",
					Image:   "ubuntu:latest",
					Command: []string{"/bin/bash", "-c", "trap 'exit 0' SIGTERM; while true; do sleep 1; done"},
					Stdin:   true,
					TTY:     true,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	_, err := client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	return err
}


func createPodWithRetry(ctx context.Context, client *kubernetes.Clientset, podName string, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := createPod(ctx, client, podName)
			if err == nil {
				return nil
			}
			lastErr = err
			if statusErr, ok := err.(*errors.StatusError); ok && 
			   statusErr.ErrStatus.Reason == metav1.StatusReasonAlreadyExists {
				podName = generatePodName(podName)
				continue
			}
			time.Sleep(1 * time.Second)
		}
	}
	return fmt.Errorf("after %d attempts, last error: %v", maxRetries, lastErr)
}

func deletePodSafe(client *kubernetes.Clientset, podName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deletePolicy := metav1.DeletePropagationBackground
	err := client.CoreV1().Pods("default").Delete(ctx, podName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		log.Printf("Error deleting pod %s: %v", podName, err)
	} else {
		log.Printf("Successfully deleted pod: %s", podName)
	}
}

func cleanupPods() {
	log.Println("Starting cleanup of orphaned pods...")
	config, err := getKubernetesConfig()
	if err != nil {
		log.Printf("Cleanup failed to get config: %v", err)
		return
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Cleanup failed to create clientset: %v", err)
		return
	}
	pods, err := clientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app=terminal-pod",
	})
	if err != nil {
		log.Printf("Cleanup failed to list pods: %v", err)
		return
	}
	for _, pod := range pods.Items {
		if _, ok := activePods.Load(pod.Name); !ok {
			log.Printf("Cleaning up orphaned pod: %s", pod.Name)
			deletePodSafe(clientset, pod.Name)
		}
	}
}

func handleTerminal(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	var msg struct {
		UserID string `json:"user_id"`
	}
	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("Failed to read user ID: %v", err)
		return
	}

	if msg.UserID == "" {
		log.Printf("Empty user_id received")
		return
	}

	config, err := getKubernetesConfig()
	if err != nil {
		log.Printf("Failed to get Kubernetes config: %v", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Failed to create clientset: %v", err)
		return
	}

	podName := generatePodName(msg.UserID)
	log.Printf("Creating pod: %s for user: %s", podName, msg.UserID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := createPodWithRetry(ctx, clientset, podName, 3); err != nil {
		log.Printf("Failed to create pod after retries: %v", err)
		return
	}

	defer func() {
		deletePodSafe(clientset, podName)
		activePods.Delete(podName)
	}()

	activePods.Store(podName, struct{}{})

	session := NewTerminalSession(conn, clientset, podName)
	defer session.Close()

	if err := session.WaitForPodRunning(ctx); err != nil {
		log.Printf("Error waiting for pod: %v", err)
		return
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace("default").
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/bash"},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Printf("Failed to create executor: %v", err)
		return
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             session,
		Stdout:            session,
		Stderr:            session,
		Tty:               true,
		TerminalSizeQueue: session,
	})

	if err != nil {
		log.Printf("Terminal session error: %v", err)
	}
}

func main() {
	cleanupPods()

	server := &http.Server{
		Addr:    serverPort,
		Handler: http.HandlerFunc(handleTerminal),
	}

	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		cleanupPods()
		close(done)
	}()

	log.Printf("🟢 Server started on ws://0.0.0.0%s/terminal", serverPort)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
	<-done
	log.Println("🛑 Server shutdown complete")
}