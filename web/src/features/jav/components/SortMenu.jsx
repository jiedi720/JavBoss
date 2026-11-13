import { useState } from 'react'
import { Popover } from '@mui/material'
import SwapVertIcon from '@mui/icons-material/SwapVert'
import { findSortOption, reverseSortValue } from '@/constants/jav'
import { SortText } from '@/features/jav/components/JavIdolView'
import { zh } from '@/utils/i18n'

// Compact sort dropdown used by the studio and series tab toolbars. It mirrors
// the idol sort menu so every list tab sorts the same way.
export default function SortMenu({ options, value, onChange, label, ariaLabel }) {
  const [anchorEl, setAnchorEl] = useState(null)
  const currentOption = findSortOption(options, value) || options[0]
  const isOptionActive = (option) => Boolean(findSortOption([option], value))
  const close = () => setAnchorEl(null)

  return (
    <>
      <div className="pagination-sort-group flex items-center">
        <span className="pagination-sort-label text-gray-500 dark:text-ui-faint">{label}</span>
        <button
          type="button"
          onClick={(event) => setAnchorEl(event.currentTarget)}
          aria-haspopup="dialog"
          aria-expanded={Boolean(anchorEl)}
          aria-label={ariaLabel}
          className="pagination-sort-button"
        >
          <SortText option={currentOption} value={value} />
          <span aria-hidden="true" className="pagination-sort-caret" />
        </button>
      </div>
      <Popover
        open={Boolean(anchorEl)}
        anchorEl={anchorEl}
        onClose={close}
        disableScrollLock
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <div className="pagination-sort-menu">
          {options.map((option) => {
            const active = isOptionActive(option)
            const displayValue = active ? value : option.defaultValue
            return (
              <div
                key={option.base}
                className={`pagination-sort-row ${
                  active
                    ? 'bg-blue-50 text-ui-primary-hover dark:bg-ui-blue-surface dark:text-blue-300'
                    : 'text-gray-700 hover:bg-gray-50 dark:text-ui-secondary dark:hover:bg-ui-hover'
                }`}
              >
                <button
                  type="button"
                  onClick={() => {
                    close()
                    onChange(displayValue)
                  }}
                  className="pagination-sort-option"
                >
                  <SortText option={option} value={displayValue} />
                </button>
                <button
                  type="button"
                  onClick={() => {
                    close()
                    onChange(reverseSortValue([option], displayValue, option.defaultValue))
                  }}
                  className="pagination-sort-reverse"
                  title={zh('反转排序', 'Reverse sort')}
                  aria-label={zh(`反转${option.label[0]}排序`, `Reverse ${option.label[1]} sort`)}
                >
                  <SwapVertIcon fontSize="inherit" />
                </button>
              </div>
            )
          })}
        </div>
      </Popover>
    </>
  )
}
