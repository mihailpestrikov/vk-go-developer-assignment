package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "vk-go-developer-assignment/api/proto"
)

func main() {
	var (
		serverAddr  = flag.String("addr", "localhost:50051", "Адрес gRPC сервера")
		action      = flag.String("action", "subscribe", "Действие: subscribe, publish или both")
		key         = flag.String("key", "test-channel", "Ключ для подписки/публикации")
		message     = flag.String("message", "Test message", "Сообщение для публикации")
		publishFreq = flag.Duration("freq", 3*time.Second, "Частота публикации сообщений (для 'both')")
	)
	flag.Parse()

	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к серверу: %v", err)
	}
	defer conn.Close()

	client := pb.NewPubSubClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\nПолучен сигнал завершения, закрываем соединение...")
		cancel()
		os.Exit(0)
	}()

	switch *action {
	case "subscribe":
		subscribe(ctx, client, *key)
	case "publish":
		publish(ctx, client, *key, *message)
	case "both":
		go publishPeriodically(ctx, client, *key, *message, *publishFreq)
		subscribe(ctx, client, *key)
	default:
		log.Fatalf("Неизвестное действие: %s", *action)
	}
}

func subscribe(ctx context.Context, client pb.PubSubClient, key string) {
	fmt.Printf("Подписка на канал: %s\n", key)

	req := &pb.SubscribeRequest{Key: key}
	stream, err := client.Subscribe(ctx, req)
	if err != nil {
		log.Fatalf("Ошибка при подписке: %v", err)
	}

	fmt.Println("Подписка успешно создана! Ожидание сообщений...")
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Fatalf("Ошибка при получении сообщения: %v", err)
		}
		fmt.Printf("Получено сообщение: %s\n", event.Data)
	}
}

func publish(ctx context.Context, client pb.PubSubClient, key, message string) {
	fmt.Printf("Публикация сообщения в канал: %s\nСообщение: %s\n", key, message)

	req := &pb.PublishRequest{
		Key:  key,
		Data: message,
	}

	_, err := client.Publish(ctx, req)
	if err != nil {
		log.Fatalf("Ошибка при публикации: %v", err)
	}

	fmt.Println("Сообщение успешно опубликовано!")
}

func publishPeriodically(ctx context.Context, client pb.PubSubClient, key, messageBase string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	count := 1
	for {
		select {
		case <-ticker.C:
			message := fmt.Sprintf("%s #%d [%s]", messageBase, count, time.Now().Format(time.RFC3339))
			publish(ctx, client, key, message)
			count++
		case <-ctx.Done():
			return
		}
	}
}
