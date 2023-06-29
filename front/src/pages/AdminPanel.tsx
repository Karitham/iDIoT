import { FunctionComponent, useState, useCallback, useEffect } from 'react'
import UserAccount, { Account } from '../components/UserAccount'
import PortalPopup from '../components/PortalPopup'
import { useNavigate } from 'react-router-dom'
import AdminPanelContainer, { AdminPanelContainerProps } from '../components/AdminPanelContainer'
import AdminPanelDeviceContainer from '../components/AdminPanelDeviceContainer'
import '../styles/AdminPanel.css'
import Navbar from '../components/Navbar'
import AdminPlanningContainer from '../components/AdminPlanningContainer'
import { client } from '../api/client'
import SubmitButton from '../components/SubmitButton'
import useAuth from '../api/useAuth'

const device1 = { src: '/clap.svg', name: 'Living Room Cam 1' }
const device2 = { src: '/spinner.svg', name: 'Living Room Cam 2' }
const device3 = { src: '/checkbi.svg', name: 'Bedroom Fire Captor' }
const device4 = { src: '/skull.svg', name: 'Bedroom Cam' }

const devices = [device1, device2, device3, device4]

const AdminPanel: FunctionComponent = () => {
  const [isUserAccountOpen, setUserAccountOpen] = useState(false)
  const [users, setUsers] = useState<AdminPanelContainerProps>()
  const navigate = useNavigate()

  const openUserAccount = useCallback(() => {
    setUserAccountOpen(true)
  }, [])

  const closeUserAccount = useCallback(() => {
    setUserAccountOpen(false)
  }, [])

  const onLogoutContainerClick = useCallback(() => {
    const destroySession = useAuth()[2]
    destroySession()
    navigate('/')
  }, [navigate])

  useEffect(() => {
    refreshAccounts(setUsers)
  }, [setUsers])

  return (
    <>
      <div className="admin-panel">
        <Navbar settingsText="Settings" showBreadcrumbs={true} />
        <div className="container contentwrapper1">
          <div className="titlecontainer">
            <div className="titre">Settings</div>
          </div>
          <AdminPlanningContainer />
          {users && <AdminPanelContainer accounts={users.accounts} />}
          <div className="devices">
            <div className="devices1">Devices</div>
            <div className="accounts1">
              <div className="device-list">
                {devices.map(d => (
                  <AdminPanelDeviceContainer src={d.src} name={d.name} />
                ))}
              </div>
            </div>
          </div>
          <SubmitButton className="new-user" onClick={openUserAccount} text="New user"></SubmitButton>
          <SubmitButton className="logout" onClick={onLogoutContainerClick} text="Log out"></SubmitButton>
        </div>
      </div>
      {isUserAccountOpen && (
        <PortalPopup overlayColor="rgba(113, 113, 113, 0.3)" placement="Centered" onOutsideClick={closeUserAccount}>
          <UserAccount
            onClose={closeUserAccount}
            onAccountSave={t => {
              createUserAccount(t).then(() => refreshAccounts(setUsers))
              closeUserAccount()
            }}
          />
        </PortalPopup>
      )}
    </>
  )
}

export default AdminPanel

const createUserAccount = async (user: Account) => {
  const response = await client.post('/users', {
    body: {
      name: user.name,
      password: user.password!,
      email: user.email
    }
  })

  if (response.data) {
    console.log('User account created:', response.data)
  }

  return response.data
}

const refreshAccounts = async (setUsers: (a: AdminPanelContainerProps) => void) => {
  try {
    const response = await client.get('/users', {})
    if (response.data) {
      setUsers({ accounts: response.data })
      console.log('Accounts:', response.data)
    } else {
      console.error('Error fetching accounts: Response data is undefined')
    }
  } catch (error) {
    console.error('Error fetching accounts:', error)
  }
}
