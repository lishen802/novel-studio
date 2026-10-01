import { cleanup, render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import {afterEach,it,vi,expect} from 'vitest';
import Versions from './Versions';
import {AppProvider} from '../store/AppContext';
afterEach(()=>{cleanup();delete window.go;});
it('导出确认后创建真实文件并显示结果，不把占位按钮当成功',async()=>{
 const exportFile=vi.fn().mockResolvedValue({path:'/book/exports/manuscript.txt',chapterCount:2,draftCount:1});
 window.go={main:{DesktopAPI:{ApplicationBootstrap:vi.fn().mockResolvedValue({preferences:{},providers:[],profiles:[],recentProjects:[]}),Snapshots:vi.fn().mockResolvedValue([]),ExportManuscript:exportFile}}};
 render(<MemoryRouter><AppProvider><Versions/></AppProvider></MemoryRouter>);
 fireEvent.click(await screen.findByRole('button',{name:'导出为纯文本'}));expect(exportFile).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole('button',{name:'确认导出'}));expect(await screen.findByLabelText('导出文件路径')).toBeTruthy();
 expect(exportFile).toHaveBeenCalledWith('plain');expect(screen.getByText(/已导出 2 章/)).toBeTruthy();
});
