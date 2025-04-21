import asyncio
import websockets

# Обработчик сообщений от клиента
async def echo(websocket, path):
    async for message in websocket:
        print(f"Сообщение от клиента: {message}")
        await websocket.send(f"Ответ от сервера: {message}")

# Запуск WebSocket-сервера
async def start_server():
    server = await websockets.serve(echo, "0.0.0.0", 8765)  # Прослушиваем на всех интерфейсах
    print("Сервер запущен на ws://0.0.0.0:8765")
    await server.wait_closed()

# Запуск сервера
if __name__ == "__main__":
    asyncio.run(start_server())
