/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import { from } from 'rxjs';
import { SnackbarStatus, UpdateSnackbar } from '../../utils/store';
import { Compliance } from './Compliance';
import { fireEvent, render } from '@testing-library/react';
import { Spy } from 'spy4js';
import { act } from 'react';

const mockStreamDeploymentHistory = Spy('StreamDeploymentHistory');
const mockSaveFile = jest.fn();

jest.mock('../../utils/GrpcApi', () => ({
    get useApi() {
        return {
            overviewService: () => ({
                StreamDeploymentHistory: () => mockStreamDeploymentHistory(),
            }),
        };
    },
}));

describe('Compliance', () => {
    const getNode = () => <Compliance saveFile={mockSaveFile} />;
    const getWrapper = () => render(getNode());

    it('shows an error with no environment', () => {
        const { container } = getWrapper();
        const downloadButton = container.querySelector('button');
        act(() => {
            downloadButton?.click();
        });
        expect(UpdateSnackbar.get().show).toBe(true);
        expect(UpdateSnackbar.get().status).toBe(SnackbarStatus.ERROR);
        expect(UpdateSnackbar.get().content).toBe(
            'Cannot download deployment history without an environment selected.'
        );
    });

    it('shows an error with no dates selected', () => {
        const { container } = getWrapper();
        const downloadButton = container.querySelector('button');
        const environmentSelect = container.querySelector('select');

        act(() => {
            if (environmentSelect) fireEvent.change(environmentSelect, { target: { value: 'test/test' } });
            downloadButton?.click();
        });

        expect(UpdateSnackbar.get().show).toBe(true);
        expect(UpdateSnackbar.get().status).toBe(SnackbarStatus.ERROR);
        expect(UpdateSnackbar.get().content).toBe('Cannot download deployment history without a start and end date.');
    });

    it('shows an error with an end date from before the start date', () => {
        const { container } = getWrapper();
        const downloadButton = container.querySelector('button');
        const startDate = container.querySelector('input#start-date');
        const endDate = container.querySelector('input#end-date');
        const environmentSelect = container.querySelector('select');

        act(() => {
            if (environmentSelect) fireEvent.change(environmentSelect, { target: { value: 'test/test' } });
            if (endDate instanceof HTMLInputElement) fireEvent.change(endDate, { target: { value: '2001-12-09' } });
            if (startDate instanceof HTMLInputElement) fireEvent.change(startDate, { target: { value: '2025-01-20' } });

            downloadButton?.click();
        });

        expect(UpdateSnackbar.get().show).toBe(true);
        expect(UpdateSnackbar.get().status).toBe(SnackbarStatus.ERROR);
        expect(UpdateSnackbar.get().content).toBe('Cannot have an end date that happens before the start date.');
    });

    it('downloads the file received by the server', () => {
        const { container } = getWrapper();
        const downloadButton = container.querySelector('button');
        const startDate = container.querySelector('input#start-date');
        const endDate = container.querySelector('input#end-date');
        const environmentSelect = container.querySelector('select');

        const content = ['test', 'test2'];
        mockStreamDeploymentHistory.returns(from(content.map((line) => ({ deployment: line }))));

        act(() => {
            if (environmentSelect) fireEvent.change(environmentSelect, { target: { value: 'test/test' } });
            if (endDate instanceof HTMLInputElement) fireEvent.change(endDate, { target: { value: '2025-01-21' } });
            if (startDate instanceof HTMLInputElement) fireEvent.change(startDate, { target: { value: '2025-01-20' } });

            downloadButton?.click();
        });

        expect(mockSaveFile).toHaveBeenCalledWith(content);
    });
});
