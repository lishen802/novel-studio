import {cleanup,render,screen,fireEvent} from '@testing-library/react';
import {afterEach,it,expect,vi} from 'vitest';
import {CreationProgress} from './CreationProgress';
afterEach(cleanup);
it('用中文展示真实阶段和已完成步骤，不推测百分比',()=>{
 render(<CreationProgress status="running" stage="story_planning" steps={[{stage:'context_preparing',status:'completed'},{stage:'story_planning',status:'running'}]}/>);
 expect(screen.queryByText(/story_planning/)).toBeNull();expect(screen.getByText('已完成 1 个步骤')).toBeTruthy();
 expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBeNull();
});
it('未知阶段不泄漏内部标识，取消失败仍保留运行状态',async()=>{
 render(<CreationProgress status="running" stage="internal_node_xyz" onCancel={vi.fn().mockRejectedValue(new Error('网络暂不可用'))}/>);
 expect(screen.queryByText(/internal_node_xyz/)).toBeNull();fireEvent.click(screen.getByRole('button',{name:'取消运行'}));await screen.findByRole('alert');expect(screen.getByRole('progressbar')).toBeTruthy();
});
it('完成与中断不会继续显示运行进度动画',()=>{
 const {rerender}=render(<CreationProgress status="completed"/>);expect(screen.getByText('本次创作已完成')).toBeTruthy();expect(screen.queryByRole('progressbar')).toBeNull();
 rerender(<CreationProgress status="outcome_unknown"/>);expect(screen.getByText('运行状态待确认')).toBeTruthy();
});
