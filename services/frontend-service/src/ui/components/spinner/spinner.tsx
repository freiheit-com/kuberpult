/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import { useRef, useEffect, useCallback } from 'react';
import { MDCCircularProgress } from '@material/circular-progress';

export const Spinner = () => {
    const MDComponent = useRef<MDCCircularProgress>();
    const control = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCCircularProgress(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    const circleGraphic = useCallback(
        (strokeWidth: number) => (
            <svg
                className="mdc-circular-progress__indeterminate-circle-graphic"
                viewBox="0 0 144 144"
                xmlns="http://www.w3.org/2000/svg">
                <circle
                    cx="72"
                    cy="72"
                    r="54"
                    strokeDasharray={339.291}
                    strokeDashoffset={169.647}
                    strokeWidth={strokeWidth}
                />
            </svg>
        ),
        []
    );

    return (
        <div
            className="mdc-circular-progress mdc-circular-progress--indeterminate"
            role="progressbar"
            aria-label="Spinner"
            aria-valuemin={0}
            aria-valuemax={1}>
            <div className="mdc-circular-progress__indeterminate-container">
                <div className="mdc-circular-progress__spinner-layer">
                    <div className="mdc-circular-progress__circle-clipper mdc-circular-progress__circle-left">
                        {circleGraphic(8)}
                    </div>
                    <div className="mdc-circular-progress__gap-patch">{circleGraphic(6.4)}</div>
                    <div className="mdc-circular-progress__circle-clipper mdc-circular-progress__circle-right">
                        {circleGraphic(8)}
                    </div>
                </div>
            </div>
        </div>
    );
};
