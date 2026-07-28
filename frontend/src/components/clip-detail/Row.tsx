// แถวป้าย/ค่า ที่แท็บภาพรวมกับแท็บเผยแพร่ใช้ร่วมกัน — สองแท็บนี้อยู่หน้าเดียวกัน
// ตารางจึงต้องกว้างเท่ากันและมีเส้นคั่นแบบเดียวกัน ค่าที่ว่างไม่แสดงแถวเลย
export function Row({
  label, value, pre = false,
}: {
  label: string;
  value: string | number | null | undefined;
  pre?: boolean;
}) {
  if (value === null || value === undefined || value === '') return null;
  return (
    <div className="flex gap-3 py-2 border-b last:border-0 text-sm">
      <span className="text-muted-foreground w-32 shrink-0">{label}</span>
      <span className={`min-w-0 break-words${pre ? ' whitespace-pre-wrap' : ''}`}>{value}</span>
    </div>
  );
}
