import { Card, CardContent } from '../ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { MetricTooltip } from './metric-tooltip'
import { formatNum } from '../../lib/format'

export interface CategoryScore {
  category: string
  avg_percentile: number
  avg_views: number
  n: number
}

export function TopicPerformance({ scores }: { scores: CategoryScore[] }) {
  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5">
        <h2 className="text-sm font-semibold">หัวข้อไหนทำยอดดี</h2>
        <MetricTooltip text="อันดับหมวดหัวข้อตามยอดวิวจริง 30 วันล่าสุด — ข้อมูลชุดเดียวกับที่ AI ใช้เลือกหัวข้อคลิปถัดไป (แสดงเฉพาะหมวดที่มีอย่างน้อย 3 คลิป)" />
      </div>
      <Card>
        <CardContent className="pt-4">
          {scores.length === 0 ? (
            <p className="text-sm text-muted-foreground py-2">
              ยังมีข้อมูลไม่พอ — ต้องมีอย่างน้อย 3 คลิปต่อหมวดจึงจะจัดอันดับได้
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>หมวดหัวข้อ</TableHead>
                  <TableHead>ยอดวิวเฉลี่ย/คลิป</TableHead>
                  <TableHead>คะแนนเทียบคลิปอื่น</TableHead>
                  <TableHead>จำนวนคลิป</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {scores.map(s => (
                  <TableRow key={s.category}>
                    <TableCell className="font-medium">{s.category}</TableCell>
                    <TableCell>{formatNum(Math.round(s.avg_views))}</TableCell>
                    <TableCell>{Math.round(s.avg_percentile * 100)} / 100</TableCell>
                    <TableCell>{s.n}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
