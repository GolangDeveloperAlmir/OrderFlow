import { useEffect, useState } from "react";

interface Order {
  id: string;
  item: string;
  quantity: number;
}

export default function App() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [item, setItem] = useState("");
  const [quantity, setQuantity] = useState(1);

  const load = async () => {
    await fetch("/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: "demo", password: "demo" }),
    });
    const res = await fetch("/orders");
    if (res.ok) {
      setOrders(await res.json());
    }
  };

  useEffect(() => {
    load();
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    await fetch("/orders", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ item, quantity }),
    });
    setItem("");
    setQuantity(1);
    load();
  };

  return (
    <div>
      <h1>OrderFlow</h1>
      <form onSubmit={submit}>
        <input value={item} onChange={(e) => setItem(e.target.value)} placeholder="item" />
        <input type="number" value={quantity} onChange={(e) => setQuantity(Number(e.target.value))} />
        <button type="submit">Create</button>
      </form>
      <ul>
        {orders.map((o) => (
          <li key={o.id}>{o.item} - {o.quantity}</li>
        ))}
      </ul>
    </div>
  );
}
