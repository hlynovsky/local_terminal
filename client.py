import asyncio
import websockets

# Функция для подключения клиента к серверу и отправки сообщения
async def connect_to_server():
    uri = "ws://127.0.0.1:8765"  # Адрес сервера
    async with websockets.connect(uri) as websocket:
        await websocket.send("Привет, сервер!")
        response = await websocket.recv()
        print(f"Ответ от сервера: {response}")

# Запуск клиента
if __name__ == "__main__":
    asyncio.run(connect_to_server())
